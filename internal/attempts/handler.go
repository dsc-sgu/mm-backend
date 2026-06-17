package attempt

import (
	"context"
	"fmt"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

type Handler struct {
	svc     *Service
	taskSvc *tasks.Service
}

func NewHandler(svc *Service, taskSvc *tasks.Service) *Handler {
	return &Handler{svc, taskSvc}
}

type GetDiffInput struct {
	ID1 string `query:"id1"`
	ID2 string `query:"id2"`
}

type GetDiffOutput struct {
	Body []string
}

func (h *Handler) GetDiff(ctx context.Context, input *GetDiffInput) (*GetDiffOutput, error) {
	id1, err := uuid.Parse(input.ID1)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing id1: " + err.Error())
	}

	id2, err := uuid.Parse(input.ID2)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing id2: " + err.Error())
	}

	diff, err := h.svc.GetDiff(id1, id2)
	if err != nil {
		return nil, huma.Error400BadRequest("cannot make diff: " + err.Error())
	}

	return &GetDiffOutput{Body: diff}, nil
}

type PushAttemptInput struct {
	CourseID     string `query:"courseID"`
	TaskGroupID  string `query:"taskGroupID"`
	TaskPosition string `query:"taskPosition"`
	RawBody      []byte
}

type PushAttemptOutput struct {
	Body struct {
		CommitHash string `json:"commitHash"`
	}
}

func (h *Handler) PushAttempt(ctx context.Context, input *PushAttemptInput) (*PushAttemptOutput, error) {
	courseID, err := uuid.Parse(input.CourseID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing courseID: " + err.Error())
	}

	taskGroupID, err := uuid.Parse(input.TaskGroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing taskGroupID: " + err.Error())
	}

	taskPosition := 1
	if input.TaskPosition != "" {
		taskPosition, err = strconv.Atoi(input.TaskPosition)
		if err != nil || taskPosition < 1 {
			return nil, huma.Error400BadRequest("invalid taskPosition")
		}
	}

	participantID := session.UserIDFromContext(ctx)
	if participantID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	if taskPosition > 1 {
		ok, err := h.taskSvc.HasSubmittedAttempt(ctx, participantID, taskGroupID, taskPosition-1)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		if !ok {
			return nil, huma.Error400BadRequest(
				fmt.Sprintf("task %d not completed yet. complete task %d first.", taskPosition-1, taskPosition-1),
			)
		}
	}

	task, err := h.taskSvc.GetTaskByPosition(ctx, taskGroupID, taskPosition)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("task not found at position %d: %s", taskPosition, err.Error()))
	}

	commitHash, err := h.svc.PushAttempt(courseID, taskGroupID, task.ID, participantID, input.RawBody)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	out := &PushAttemptOutput{}
	out.Body.CommitHash = commitHash
	return out, nil
}

type GetAttemptsInput struct {
	TaskID        string `path:"task_id"`
	ParticipantID string `path:"participant_id"`
}

type GetAttemptsOutput struct {
	Body []Attempt
}

func (h *Handler) GetAttempts(ctx context.Context, input *GetAttemptsInput) (*GetAttemptsOutput, error) {
	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing task_id: " + err.Error())
	}

	participantID, err := uuid.Parse(input.ParticipantID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing participant_id: " + err.Error())
	}

	attempts, err := h.svc.GetAttempts(taskID, participantID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetAttemptsOutput{Body: attempts}, nil
}
