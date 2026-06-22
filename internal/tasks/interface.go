package tasks

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	// TaskGroups
	CreateTaskGroup(ctx context.Context, model *CreateTaskGroup) (*TaskGroup, error)
	GetTaskGroupByID(ctx context.Context, id uuid.UUID) (*TaskGroup, error)
	GetTaskGroupByName(ctx context.Context, name string, courseID uuid.UUID) (*TaskGroup, error)
	UpdateTaskGroup(ctx context.Context, id uuid.UUID, update *UpdateTaskGroup) (*TaskGroup, error)
	DeleteTaskGroup(ctx context.Context, id uuid.UUID) error

	// Tasks
	CreateTask(ctx context.Context, model *CreateTask) (*Task, error)
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	GetTasks(ctx context.Context, taskGroupID uuid.UUID) ([]*Task, error)
	UpdateTask(ctx context.Context, taskID uuid.UUID, update *UpdateTask) (*Task, error)
	DeleteTask(ctx context.Context, taskID uuid.UUID) error
	GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error)
}
