package attempt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type Service struct {
	repo       Repo
	git        GitManager
	tasks      TaskReader
	courses    CourseReader
	identities IdentityReader
}

func NewService(repo Repo, git GitManager, tasks TaskReader, courses CourseReader, identities IdentityReader) *Service {
	return &Service{repo: repo, git: git, tasks: tasks, courses: courses, identities: identities}
}

func (s *Service) GetAttempts(taskID, participantID uuid.UUID) ([]Attempt, error) {
	return s.repo.GetAttempts(taskID, participantID)
}

// ErrNotCourseMember is returned when a participant may not touch a course's repositories.
var ErrNotCourseMember = errors.New("not a member of the course")

// assertMember is the single authorization gate for repository access; the HTTP
// and the SSH entry points both go through it.
func (s *Service) assertMember(ctx context.Context, participantID, courseID uuid.UUID) error {
	member, err := s.courses.IsCourseMember(ctx, participantID, courseID)
	if err != nil {
		return fmt.Errorf("check course membership: %w", err)
	}
	if !member {
		return ErrNotCourseMember
	}
	return nil
}

// repoForTask resolves the repository holding a participant's attempts at a task.
// The course comes from the task itself, so a caller cannot point an attempt at a
// course the task does not belong to.
func (s *Service) repoForTask(ctx context.Context, taskID, participantID uuid.UUID) (pkggit.RepoID, error) {
	id, err := s.repo.RepoForTask(ctx, taskID, participantID)
	if err != nil {
		return pkggit.RepoID{}, err
	}
	if err := s.assertMember(ctx, id.ParticipantID, id.CourseID); err != nil {
		return pkggit.RepoID{}, err
	}
	return id, nil
}

// recordAttempt is the shared tail of every submission: whatever transport
// produced the commit, the rules that govern accepting it and storing it live
// here and nowhere else. Access is already settled by the caller's gate
// (repoForTask over HTTP, authRepo over SSH), which runs before any work.
func (s *Service) recordAttempt(ctx context.Context, id pkggit.RepoID, taskID uuid.UUID, commitHash string) error {
	if err := s.repo.SaveAttempt(id, taskID, commitHash); err != nil {
		return fmt.Errorf("save attempt: %w", err)
	}
	return nil
}

func (s *Service) PushAttempt(ctx context.Context, taskID, participantID uuid.UUID, zipData []byte) (string, error) {
	id, err := s.repoForTask(ctx, taskID, participantID)
	if err != nil {
		return "", err
	}
	files, err := pkggit.UnzipFiles(zipData)
	if err != nil {
		return "", fmt.Errorf("extract zip: %w", err)
	}
	patterns, err := s.tasks.GetTaskPatternsByTaskID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task patterns: %w", err)
	}
	if len(patterns) > 0 {
		matched := false
		for _, file := range files {
			if pkggit.MatchesAnyPattern(file.FileName, patterns) {
				matched = true
				break
			}
		}
		if !matched {
			return "", fmt.Errorf("no uploaded files match required patterns for this task (%v)", patterns)
		}
	}
	if err := s.git.EnsureRepo(id); err != nil {
		return "", fmt.Errorf("init repository: %w", err)
	}
	if err := s.tasks.RefreshRepositoryPatterns(ctx, id); err != nil {
		zap.L().Warn("refresh repository patterns", zap.Error(err))
	}
	hash, err := s.git.PushFiles(id, files)
	if err != nil {
		return "", err
	}
	if err := s.recordAttempt(ctx, id, taskID, hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Service) GetDiff(ctx context.Context, id1, id2 uuid.UUID) ([]string, error) {
	one, err := s.repo.GetAttemptCommitInfo(id1)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	two, err := s.repo.GetAttemptCommitInfo(id2)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	if one.UserID != two.UserID {
		return nil, errors.New("attempts belong to different users")
	}
	patterns, _ := s.tasks.GetTaskPatternsByTaskID(ctx, one.TaskID)
	id := pkggit.RepoID{CourseID: one.CourseID, TaskGroupID: one.TaskGroupID, ParticipantID: one.UserID}
	return s.git.Diff(id, one.CommitHash, two.CommitHash, patterns)
}

// SSHMiddleware serves Git repositories over SSH: it resolves the raw
// course/group SSH path to a RepoID, checks course membership, runs the
// git command, and records pushed attempts. Repos are stored under repoDir.
func (s *Service) SSHMiddleware(repoDir string) wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			cmd := sess.Command()
			if len(cmd) != 2 {
				sh(sess)
				return
			}

			gc := cmd[0]
			pk := sess.PublicKey()
			rawRepo := cmd[1]

			repo, err := s.repoRename(sess.Context(), rawRepo, pk)
			if err != nil {
				log.Error("repo rename failed", "error", err, "repo", rawRepo)
				pkggit.Fatal(sess, err)
				return
			}

			access := s.authRepo(sess.Context(), rawRepo, repo, pk)
			switch pkggit.GitCmd(gc) {
			case pkggit.GitReceivePack:
				switch access {
				case pkggit.ReadWriteAccess, pkggit.AdminAccess:
					if err := pkggit.GitPack(sess, gc, repoDir, repo); err != nil {
						log.Error("git push failed", "error", err, "repo", rawRepo)
						pkggit.Fatal(sess, pkggit.ErrSystemMalfunction)
					} else {
						s.onPush(sess.Context(), rawRepo, repo, pk)
					}
				default:
					pkggit.Fatal(sess, pkggit.ErrNotAuthed)
				}
			case pkggit.GitUploadPack, pkggit.GitUploadArchive:
				switch access {
				case pkggit.ReadOnlyAccess, pkggit.ReadWriteAccess, pkggit.AdminAccess:
					if err := pkggit.GitPack(sess, gc, repoDir, repo); err != nil {
						switch {
						case errors.Is(err, pkggit.ErrInvalidRepo):
							pkggit.Fatal(sess, pkggit.ErrInvalidRepo)
						default:
							log.Error("unknown git error", "error", err)
							pkggit.Fatal(sess, pkggit.ErrSystemMalfunction)
						}
					} else {
						s.onFetch(sess.Context(), rawRepo, repo, pk)
					}
				default:
					pkggit.Fatal(sess, pkggit.ErrNotAuthed)
				}
			default:
				sh(sess)
			}
		}
	}
}

func (s *Service) repoRename(ctx context.Context, original string, key gossh.PublicKey) (string, error) {
	id, err := s.GetRepoID(ctx, original, gossh.FingerprintSHA256(key))
	if err != nil {
		return "", err
	}
	if err = s.git.EnsureRepo(id); err != nil {
		return "", err
	}
	if err = s.tasks.RefreshRepositoryPatterns(ctx, id); err != nil {
		zap.L().Warn("refresh repository patterns", zap.Error(err))
	}
	return id.IntoPath() + ".git", nil
}

func (s *Service) authRepo(ctx context.Context, original, repo string, key ssh.PublicKey) pkggit.AccessLevel {
	id, err := s.GetRepoID(ctx, original, gossh.FingerprintSHA256(key))
	if err != nil {
		return pkggit.NoAccess
	}
	if err := s.assertMember(ctx, id.ParticipantID, id.CourseID); err != nil {
		return pkggit.NoAccess
	}
	return pkggit.ReadWriteAccess
}

func (s *Service) onPush(ctx context.Context, original, repo string, key ssh.PublicKey) {
	id, err := s.GetRepoID(ctx, original, gossh.FingerprintSHA256(key))
	if err != nil {
		zap.L().Error("resolve pushed repository", zap.Error(err))
		return
	}
	base := s.git.RepoPath(id)
	options, err := os.ReadFile(filepath.Join(base, "push-options"))
	if err != nil {
		return
	}
	defer os.Remove(filepath.Join(base, "push-options"))
	name := parseSubmitOption(strings.Split(strings.TrimSpace(string(options)), "\n"))
	if name == "" {
		return
	}
	taskID, err := s.tasks.GetTaskByName(ctx, id.TaskGroupID, name)
	if err != nil {
		return
	}
	tags, err := os.ReadFile(filepath.Join(base, "push-tags"))
	if err != nil {
		return
	}
	defer os.Remove(filepath.Join(base, "push-tags"))
	for _, hash := range strings.Fields(string(tags)) {
		if err := s.recordAttempt(ctx, id, taskID, hash); err != nil {
			zap.L().Error("save pushed attempt", zap.Error(err))
		}
	}
}

func (s *Service) onFetch(context.Context, string, string, ssh.PublicKey) {}

func (s *Service) GetRepoID(ctx context.Context, path, fingerprint string) (pkggit.RepoID, error) {
	parts := strings.Split(strings.Trim(strings.TrimSuffix(path, ".git"), string(os.PathSeparator)), string(os.PathSeparator))
	if len(parts) < 2 {
		return pkggit.RepoID{}, errors.New("invalid path: need course/group_name")
	}
	courseID, err := s.courses.GetCourse(ctx, parts[0])
	if err != nil {
		return pkggit.RepoID{}, err
	}
	groupID, err := s.tasks.GetTaskGroupIDByName(ctx, parts[1], courseID)
	if err != nil {
		return pkggit.RepoID{}, err
	}
	participantID, err := s.identities.ParticipantID(fingerprint)
	if err != nil {
		return pkggit.RepoID{}, err
	}
	return pkggit.RepoID{CourseID: courseID, TaskGroupID: groupID, ParticipantID: participantID}, nil
}

func parseSubmitOption(options []string) string {
	for _, option := range options {
		if value, ok := strings.CutPrefix(strings.TrimSpace(option), "submit="); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
