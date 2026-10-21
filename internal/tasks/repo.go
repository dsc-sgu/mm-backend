package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type TaskGroup struct {
	ID        uuid.UUID  `json:"id"       db:"id"         binding:"required"`
	CourseID  uuid.UUID  `json:"courseID" db:"course_id"  binding:"required"`
	Name      string     `json:"name"     db:"name"       binding:"required"`
	DeletedAt *time.Time `json:"-"        db:"deleted_at"`
}

// Task is a task as a reader sees it: its identity from the tasks table and
// its settings from the version of its block the reader sees (their own
// draft for a teacher, else the published one). BlockID and Version name
// that block version; an edit is sent from Version.
type Task struct {
	ID          uuid.UUID       `json:"id"                  binding:"required"`
	TaskGroupID uuid.UUID       `json:"taskGroupID"         binding:"required"`
	Name        string          `json:"name"                binding:"required"`
	BlockID     uuid.UUID       `json:"blockID"             binding:"required"`
	Version     int             `json:"version"             binding:"required"`
	Draft       bool            `json:"draft"               binding:"required"`
	Statement   json.RawMessage `json:"statement,omitempty"`
	Patterns    []string        `json:"patterns"`
	MaxGrade    float32         `json:"maxGrade"            binding:"required"`
	MaxAttempts *int            `json:"maxAttempts"`
	AvailableAt *time.Time      `json:"availableAt"`
	DeadlineAt  *time.Time      `json:"deadlineAt"`
}

// TaskUpdate is the input for patching a task's settings. Version is the
// block version the author loaded, as for a block edit. TaskGroupID and
// Name are immutable after creation, so they have no place here.
type TaskUpdate struct {
	Version     int                    `json:"version"              minimum:"1"`
	Statement   json.RawMessage        `json:"statement,omitempty"`
	Patterns    *[]string              `json:"patterns,omitempty"`
	MaxGrade    *float32               `json:"maxGrade,omitempty"`
	MaxAttempts OmittableNullable[int] `json:"maxAttempts,omitzero" minimum:"1"`
	AvailableAt *time.Time             `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time             `json:"deadlineAt,omitempty"`
}

// UpdatedTask is the result of patching a task. Published tells whether
// the change went live at once: it did when the author had no draft of the
// task's block, so the edit became one and was published on its own;
// otherwise it joined their draft and goes out with the course.
type UpdatedTask struct {
	Task
	Published bool `json:"published"`
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
	// GetTaskByID returns the task with the settings of the block version
	// viewerID sees; with viewerID = uuid.Nil, the published one.
	GetTaskByID(ctx context.Context, taskID, viewerID uuid.UUID) (*Task, error)
	// UpdateTask applies the settings to userID's draft of the task's
	// block, edited from update.Version. A draft made by this call is
	// published on its own, with a checkpoint; an existing one keeps the
	// change until the course is published.
	UpdateTask(
		ctx context.Context,
		taskID, userID uuid.UUID,
		update *TaskUpdate,
	) (*UpdatedTask, error)
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
	// GetCourseIDByTask resolves the course of a live task, published or
	// not; ErrTaskNotFound otherwise.
	GetCourseIDByTask(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error)
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
