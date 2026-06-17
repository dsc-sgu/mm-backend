package tasks

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	// TaskGroups
	CreateTaskGroup(ctx context.Context, blockID uuid.UUID, model *CreateTaskGroup) (*TaskGroup, error)
	GetTaskGroupByBlockID(ctx context.Context, blockID uuid.UUID) (*TaskGroup, error)
	GetTaskGroupByName(ctx context.Context, name string, courseID uuid.UUID) (*TaskGroup, error)
	UpdateTaskGroup(ctx context.Context, blockID uuid.UUID, update *UpdateTaskGroup) (*TaskGroup, error)
	DeleteTaskGroup(ctx context.Context, blockID uuid.UUID) error

	// Tasks
	CreateTask(ctx context.Context, model *CreateTask) (*Task, error)
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	GetTaskByPosition(ctx context.Context, taskGroupID uuid.UUID, position int) (*Task, error)
	GetTasks(ctx context.Context, taskGroupID uuid.UUID) ([]*Task, error)
	UpdateTask(ctx context.Context, taskID uuid.UUID, update *UpdateTask) (*Task, error)
	DeleteTask(ctx context.Context, taskID uuid.UUID) error
	GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error)

	// Sequential
	HasSubmittedAttempt(ctx context.Context, userID uuid.UUID, taskGroupID uuid.UUID, position int) (bool, error)
}
