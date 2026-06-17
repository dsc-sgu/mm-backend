package tasks

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TaskGroup struct {
	BlockID uuid.UUID `json:"blockID" db:"block_id" binding:"required"`
	Name    string    `json:"name"    db:"name"     binding:"required"`
}

type Task struct {
	ID          uuid.UUID       `json:"id"          db:"id"            binding:"required"`
	TaskGroupID uuid.UUID       `json:"taskGroupID" db:"task_group_id" binding:"required"`
	Position    int             `json:"position"    db:"position"      binding:"required"`
	Data        json.RawMessage `json:"data"        db:"data"          binding:"required"`
	MaxGrade    float32         `json:"maxGrade"    db:"max_grade"     binding:"required"`
	MaxAttempts int             `json:"maxAttempts" db:"max_attempts"  binding:"required"`
	AvailableAt *time.Time      `json:"availableAt" db:"available_at"`
	DeadlineAt  *time.Time      `json:"deadlineAt"  db:"deadline_at"`
	LeadTime    *time.Time      `json:"leadTime"    db:"lead_time"`
}

type CreateTaskGroup struct {
	CourseID uuid.UUID       `json:"courseID" binding:"required"`
	Name     string          `json:"name"     binding:"required"`
	Data     json.RawMessage `json:"data"     binding:"required" swaggertype:"object"`
}

type UpdateTaskGroup struct {
	Name *string `json:"name,omitempty"`
}

type CreateTask struct {
	TaskGroupID uuid.UUID       `json:"taskGroupID,omitempty" db:"task_group_id"`
	Data        json.RawMessage `json:"data"                           binding:"required" swaggertype:"object"`
	MaxGrade    float32         `json:"maxGrade"                       binding:"required"`
	MaxAttempts int             `json:"maxAttempts"                    binding:"required"`
	AvailableAt *time.Time      `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time      `json:"deadlineAt,omitempty"`
	LeadTime    *time.Time      `json:"leadTime,omitempty"`
}

type UpdateTask struct {
	Data        *json.RawMessage `json:"data,omitempty"`
	MaxGrade    *float32         `json:"maxGrade,omitempty"`
	MaxAttempts *int             `json:"maxAttempts,omitempty"`
	AvailableAt *time.Time       `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time       `json:"deadlineAt,omitempty"`
	LeadTime    *time.Time       `json:"leadTime,omitempty"`
}

type CreateTaskGroupResponse struct {
	BlockID uuid.UUID `json:"blockID"`
}

type CreateTaskResponse struct {
	ID uuid.UUID `json:"id"`
}

type TaskGroupWithTasks struct {
	TaskGroup
	Tasks []*Task `json:"tasks"`
}
