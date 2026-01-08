package courses

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateCourse(model *CreateCourse, ownerId uuid.UUID) (*Course, error)
	GetCourseById(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(limit int, lastId uuid.UUID) ([]*Course, error)
	UpdateCourseById(id uuid.UUID, update *UpdateCourse) (*Course, error)
	DeleteCourseById(id uuid.UUID) error
}
