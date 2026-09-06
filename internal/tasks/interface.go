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
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	GetTasks(ctx context.Context, taskGroupID, snapshotID uuid.UUID) ([]*Task, error)
	GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error)
	GetTaskGroupIDByName(ctx context.Context, name string, courseID uuid.UUID) (uuid.UUID, error)
	GetCourseIDByTaskGroup(ctx context.Context, taskGroupID uuid.UUID) (uuid.UUID, error)
	GetTaskByName(ctx context.Context, taskGroupID uuid.UUID, name string) (uuid.UUID, error)
	GetTaskPatterns(ctx context.Context, taskGroupID uuid.UUID) (map[string][]string, error)
	GetTaskPatternsByTaskID(ctx context.Context, taskID uuid.UUID) ([]string, error)
	// ResolveViewSnapshot picks which snapshot generation of tasks a caller
	// should see for a course: their own in-progress draft, if any, else the
	// course's active snapshot.
	ResolveViewSnapshot(ctx context.Context, courseID, userID, sessionID uuid.UUID) (uuid.UUID, error)
}
