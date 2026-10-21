package attempt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type AttemptStatus string

type AttemptCommitInfo struct {
	UserID      uuid.UUID
	TaskID      uuid.UUID
	CommitHash  string
	CourseID    uuid.UUID
	TaskGroupID uuid.UUID
}

type Attempt struct {
	Id             uuid.UUID       `json:"id"                       db:"attempt_id"      binding:"required"`
	UserID         uuid.UUID       `json:"userId"                   db:"user_id"         binding:"required"`
	TaskID         uuid.UUID       `json:"taskId"                   db:"task_id"         binding:"required"`
	State          AttemptStatus   `json:"state"                    db:"state"           binding:"required"`
	TransitionAt   time.Time       `json:"transitionAt"             db:"transition_at"   binding:"required"`
	TransitionData json.RawMessage `json:"transitionData,omitempty" db:"transition_data"`
}

type Repo interface {
	GetAttempts(taskID, participantID uuid.UUID) ([]Attempt, error)
	SaveAttempt(repoID pkggit.RepoID, taskID uuid.UUID, commitHash string) error
	GetAttemptCommitInfo(attemptID uuid.UUID) (AttemptCommitInfo, error)
	// RepoForTask resolves which course and task group a task belongs to,
	// yielding the repository that holds the participant's attempts at it.
	RepoForTask(
		ctx context.Context,
		taskID, participantID uuid.UUID,
	) (pkggit.RepoID, error)
	// GetTaskCourseID resolves which course a task belongs to, for
	// authorizing access to already-recorded attempts.
	GetTaskCourseID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error)
}

type TaskReader interface {
	GetTaskPatternsByTaskID(context.Context, uuid.UUID) ([]string, error)
}

type CourseReader interface {
	IsCourseMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	IsCourseTeacher(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type GitManager interface {
	Lock(pkggit.RepoID) (unlock func())
	EnsureRepo(pkggit.RepoID) error
	RepoPath(pkggit.RepoID) string
	ListFiles(pkggit.RepoID) ([]string, error)
	CommitFiles(barePath string, files []pkggit.FileInfo, remove []string, message string) (string, error)
	Diff(id pkggit.RepoID, fromHash, toHash string, include func(path string) bool) ([]pkggit.ChangedFile, error)
}
