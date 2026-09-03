package attempt

import (
	"context"

	"github.com/google/uuid"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type Repo interface {
	GetAttempts(taskID, participantID uuid.UUID) ([]Attempt, error)
	SaveAttempt(repoID pkggit.RepoID, taskID uuid.UUID, commitHash string) error
	GetAttemptCommitInfo(attemptID uuid.UUID) (AttemptCommitInfo, error)
	// RepoForTask resolves which course and task group a task belongs to,
	// yielding the repository that holds the participant's attempts at it.
	RepoForTask(ctx context.Context, taskID, participantID uuid.UUID) (pkggit.RepoID, error)
}

type TaskReader interface {
	GetTaskGroupIDByName(context.Context, string, uuid.UUID) (uuid.UUID, error)
	GetTaskByName(context.Context, uuid.UUID, string) (uuid.UUID, error)
	GetTaskPatternsByTaskID(context.Context, uuid.UUID) ([]string, error)
	RefreshRepositoryPatterns(context.Context, pkggit.RepoID) error
}

type CourseReader interface {
	GetCourse(context.Context, string) (uuid.UUID, error)
	IsCourseMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type IdentityReader interface {
	ParticipantID(fingerprint string) (uuid.UUID, error)
}

type GitManager interface {
	EnsureRepo(pkggit.RepoID) error
	RepoPath(pkggit.RepoID) string
	PushFiles(pkggit.RepoID, []pkggit.FileInfo) (string, error)
	Diff(pkggit.RepoID, string, string, []string) ([]string, error)
}
