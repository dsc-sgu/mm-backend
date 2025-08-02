package courses

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateCourse, ownerId uuid.UUID) (*Course, error)
	GetById(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(limit int, offset int) ([]*Course, error)
	UpdateById(id uuid.UUID, update *UpdateCourse) (*Course, error)
	DeleteById(id uuid.UUID) error
}
