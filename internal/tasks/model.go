package tasks

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ErrTaskGroupNotFound is returned when a task group id does not resolve to
// an existing task group.
var ErrTaskGroupNotFound = errors.New("task group not found")

type TaskGroup struct {
	ID       uuid.UUID `json:"id"       db:"id"        binding:"required"`
	CourseID uuid.UUID `json:"courseID" db:"course_id" binding:"required"`
	Name     string    `json:"name"     db:"name"      binding:"required"`
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
