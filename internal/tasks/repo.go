package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
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
	MaxAttempts *int             `json:"maxAttempts"         db:"max_attempts"`
	AvailableAt *time.Time       `json:"availableAt"         db:"available_at"`
	DeadlineAt  *time.Time       `json:"deadlineAt"          db:"deadline_at"`
}

// TaskUpdate is the input for patching a task. TaskGroupID and Name are
// immutable after creation, so they have no place here.
type TaskUpdate struct {
	Statement   json.RawMessage        `json:"statement,omitempty"`
	Patterns    *[]string              `json:"patterns,omitempty"`
	MaxGrade    *float32               `json:"maxGrade,omitempty"`
	MaxAttempts OmittableNullable[int] `json:"maxAttempts,omitzero" minimum:"1"`
	AvailableAt *time.Time             `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time             `json:"deadlineAt,omitempty"`
}

// OmittableNullable is a field which can be omitted from the input, set to
// null, or set to a value, so a patch can tell "keep" from "clear". Taken
// from the huma schema customization docs.
type OmittableNullable[T any] struct {
	Sent  bool
	Null  bool
	Value T
}

func (o *OmittableNullable[T]) UnmarshalJSON(b []byte) error {
	if len(b) > 0 {
		o.Sent = true
		if bytes.Equal(b, []byte("null")) {
			o.Null = true
			return nil
		}
		return json.Unmarshal(b, &o.Value)
	}
	return nil
}

func (o OmittableNullable[T]) MarshalJSON() ([]byte, error) {
	if o.Null {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// IsZero makes an omitted field stay omitted under the omitzero tag.
func (o OmittableNullable[T]) IsZero() bool {
	return !o.Sent
}

func (o OmittableNullable[T]) Schema(r huma.Registry) *huma.Schema {
	return r.Schema(reflect.TypeOf(o.Value), true, "")
}

// Ptr returns the value, or nil if the field was null.
func (o OmittableNullable[T]) Ptr() *T {
	if o.Null {
		return nil
	}
	return &o.Value
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
