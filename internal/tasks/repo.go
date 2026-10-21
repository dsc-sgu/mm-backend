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
	ID          uuid.UUID        `json:"id"                  db:"id"            binding:"required"`
	TaskGroupID uuid.UUID        `json:"taskGroupID"         db:"task_group_id" binding:"required"`
	Name        string           `json:"name"                db:"name"          binding:"required"`
	Statement   *json.RawMessage `json:"statement,omitempty" db:"statement"`
	Patterns    pq.StringArray   `json:"patterns"            db:"patterns"`
	MaxGrade    float32          `json:"maxGrade"            db:"max_grade"     binding:"required"`
	MaxAttempts int              `json:"maxAttempts"         db:"max_attempts"  binding:"required"`
	AvailableAt *time.Time       `json:"availableAt"         db:"available_at"`
	DeadlineAt  *time.Time       `json:"deadlineAt"          db:"deadline_at"`
}

// TaskUpdate is the input for patching a task. TaskGroupID and Name are
// immutable after creation, so they have no place here.
type TaskUpdate struct {
	Statement   json.RawMessage `json:"statement,omitempty"`
	Patterns    *[]string       `json:"patterns,omitempty"`
	MaxGrade    *float32        `json:"maxGrade,omitempty"`
	MaxAttempts *int            `json:"maxAttempts,omitempty"`
	AvailableAt *time.Time      `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time      `json:"deadlineAt,omitempty"`
}

type UpdateTaskGroup struct {
	Name string `json:"name" minLength:"1" maxLength:"128"`
}

type TaskGroupWithTasks struct {
	TaskGroup
	Tasks []*Task `json:"tasks"`
}

type Repo interface {
	// TaskGroups
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

	// Tasks
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	UpdateTask(
		ctx context.Context,
		taskID uuid.UUID,
		update *TaskUpdate,
	) (*Task, error)
	// GetTasks lists the tasks of a group whose blocks are live as viewerID
	// sees the course: their own drafts over the published blocks. With
	// viewerID = uuid.Nil it is the published course.
	GetTasks(
		ctx context.Context,
		taskGroupID, viewerID uuid.UUID,
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
	GetTaskPatternsByTaskID(
		ctx context.Context,
		taskID uuid.UUID,
	) ([]string, error)
}
