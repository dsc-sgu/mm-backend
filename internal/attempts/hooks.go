package attempt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/tasks"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

// pushRefusal is a reason to reject a push that the hook shows the client.
type pushRefusal string

func (r pushRefusal) Error() string { return string(r) }

// HookHandler serves the git hooks of SSH pushes over the hook socket. The
// SSH server has already authorized the push, and the hook passes on the
// repository it was given in its environment.
func (s *Service) HookHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /"+pkggit.PreReceiveHook, s.serveHook(s.preReceive))
	mux.HandleFunc("POST /"+pkggit.PostReceiveHook, s.serveHook(s.postReceive))
	return mux
}

func (s *Service) serveHook(
	handle func(context.Context, pkggit.HookRequest) error,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pkggit.HookRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid hook request", http.StatusBadRequest)
			return
		}
		err := handle(r.Context(), request)
		var refusal pushRefusal
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.As(err, &refusal):
			http.Error(w, refusal.Error(), http.StatusConflict)
		default:
			zap.L().Error("git hook", zap.String("path", r.URL.Path), zap.Error(err))
			http.Error(w, pkggit.ErrSystemMalfunction.Error(), http.StatusInternalServerError)
		}
	}
}

// preReceive decides whether a push is accepted. Tags cannot be updated. New
// tags pushed with "-o submit=<task>" become attempts at that task, so each
// must have a file matching the task's patterns and the participant must have
// that many attempts left.
func (s *Service) preReceive(ctx context.Context, request pkggit.HookRequest) error {
	var tags []pkggit.HookRef
	for _, ref := range request.Refs {
		if !strings.HasPrefix(ref.Ref, "refs/tags/") {
			continue
		}
		if !ref.IsNewTag() {
			return pushRefusal("tag updates not allowed")
		}
		tags = append(tags, ref)
	}
	if request.Task == "" || len(tags) == 0 {
		return nil
	}

	taskID, err := s.submittedTask(ctx, request)
	if err != nil {
		return err
	}
	patterns, err := s.tasks.GetTaskPatternsByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task patterns: %w", err)
	}
	if len(patterns) > 0 {
		compiled := pkggit.CompilePatterns(patterns)
		for _, tag := range tags {
			if !matchesAny(compiled, tag.Files) {
				return pushRefusal(fmt.Sprintf(
					"no files match required patterns for task '%s'",
					request.Task,
				))
			}
		}
	}
	err = s.assertAttemptsLeft(ctx, taskID, request.Repo.ParticipantID, len(tags))
	if errors.Is(err, ErrAttemptsExhausted) {
		return pushRefusal(fmt.Sprintf("no attempts left for task '%s'", request.Task))
	}
	return err
}

// postReceive records an attempt for every new tag of an accepted push made
// with "-o submit=<task>".
func (s *Service) postReceive(ctx context.Context, request pkggit.HookRequest) error {
	if request.Task == "" {
		return nil
	}
	taskID, err := s.submittedTask(ctx, request)
	if err != nil {
		return err
	}
	for _, ref := range request.Refs {
		if !ref.IsNewTag() {
			continue
		}
		if err := s.recordAttempt(ctx, request.Repo, taskID, ref.New); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) submittedTask(
	ctx context.Context,
	request pkggit.HookRequest,
) (uuid.UUID, error) {
	id, err := s.tasks.GetTaskByName(ctx, request.Repo.TaskGroupID, request.Task)
	if errors.Is(err, tasks.ErrTaskNotFound) {
		return id, pushRefusal(fmt.Sprintf("no task named '%s' in this group", request.Task))
	}
	if err != nil {
		return id, fmt.Errorf("get task by name: %w", err)
	}
	return id, nil
}

func matchesAny(patterns pkggit.CompiledPatterns, files []string) bool {
	for _, file := range files {
		if patterns.MatchAny(file) {
			return true
		}
	}
	return false
}
