package tasks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type TaskGroup struct {
	ID        uuid.UUID  `json:"id"       db:"id"         binding:"required"`
	CourseID  uuid.UUID  `json:"courseID" db:"course_id"  binding:"required"`
	Name      string     `json:"name"     db:"name"       binding:"required"`
	DeletedAt *time.Time `json:"-"        db:"deleted_at"`
}

type Task struct {
	ID          uuid.UUID      `json:"id"          db:"block_id"      binding:"required"`
	SnapshotID  uuid.UUID      `json:"-"           db:"snapshot_id"`
	TaskGroupID uuid.UUID      `json:"taskGroupID" db:"task_group_id" binding:"required"`
	Name        string         `json:"name"        db:"name"          binding:"required"`
	Patterns    pq.StringArray `json:"patterns"    db:"patterns"`
	MaxGrade    float32        `json:"maxGrade"    db:"max_grade"     binding:"required"`
	MaxAttempts int            `json:"maxAttempts" db:"max_attempts"  binding:"required"`
	AvailableAt *time.Time     `json:"availableAt" db:"available_at"`
	DeadlineAt  *time.Time     `json:"deadlineAt"  db:"deadline_at"`
}

type CreateTaskGroup struct {
	CourseID uuid.UUID       `json:"courseID" binding:"required"`
	Name     string          `json:"name"     binding:"required"`
	Data     json.RawMessage `json:"data"     binding:"required" swaggertype:"object"`
}

type UpdateTaskGroup struct {
	Name *string `json:"name"`
}

type CreateTaskGroupResponse struct {
	ID uuid.UUID `json:"id"`
}

type TaskGroupWithTasks struct {
	TaskGroup
	Tasks []*Task `json:"tasks"`
}

type Repo interface {
	// TaskGroups
	CreateTaskGroup(
		ctx context.Context,
		model *CreateTaskGroup,
	) (*TaskGroup, error)
	GetTaskGroupByID(ctx context.Context, id uuid.UUID) (*TaskGroup, error)
	GetTaskGroupByName(
		ctx context.Context,
		name string,
		courseID uuid.UUID,
	) (*TaskGroup, error)
	UpdateTaskGroup(
		ctx context.Context,
		id uuid.UUID,
		update *UpdateTaskGroup,
	) (*TaskGroup, error)
	DeleteTaskGroup(ctx context.Context, id uuid.UUID) error

	// Tasks
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	GetTasks(
		ctx context.Context,
		taskGroupID, snapshotID uuid.UUID,
	) ([]*Task, error)
	GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error)
	GetTaskGroupIDByName(
		ctx context.Context,
		name string,
		courseID uuid.UUID,
	) (uuid.UUID, error)
	GetCourseIDByTaskGroup(
		ctx context.Context,
		taskGroupID uuid.UUID,
	) (uuid.UUID, error)
	GetTaskByName(
		ctx context.Context,
		taskGroupID uuid.UUID,
		name string,
	) (uuid.UUID, error)
	GetTaskPatterns(
		ctx context.Context,
		taskGroupID uuid.UUID,
	) (map[string][]string, error)
	GetTaskPatternsByTaskID(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]string, error)
	// ResolveViewSnapshot picks which snapshot generation of tasks a caller
	// should see for a course: their own in-progress draft, if any, else the
	// course's active snapshot.
	ResolveViewSnapshot(
		ctx context.Context,
		courseID, userID, sessionID uuid.UUID,
	) (uuid.UUID, error)
}
