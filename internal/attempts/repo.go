package attempt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type AttemptStatus string

const (
	AttemptStatusSent     AttemptStatus = "sent"
	AttemptStatusAssessed AttemptStatus = "assessed"
	AttemptStatusRetrieve AttemptStatus = "retrieve"
)

type FileInfo struct {
	FileName    string    `json:"fileName"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
}

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
	GetTaskGroupIDByName(context.Context, string, uuid.UUID) (uuid.UUID, error)
	GetTaskByName(context.Context, uuid.UUID, string) (uuid.UUID, error)
	GetTaskPatternsByTaskID(context.Context, uuid.UUID) ([]string, error)
	RefreshRepositoryPatterns(context.Context, pkggit.RepoID) error
}

type CourseReader interface {
	GetCourse(context.Context, string) (uuid.UUID, error)
	IsCourseMember(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	IsCourseTeacher(context.Context, uuid.UUID, uuid.UUID) (bool, error)
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
