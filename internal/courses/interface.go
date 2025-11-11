package courses

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateCourse(model *CreateCourse, ownerId uuid.UUID) (*Course, error)
	GetCourseById(ctx context.Context, id int) (*Course, error)
	GetPaginatedCourses(limit int, offset int) ([]*Course, error)
	UpdateCourseById(id int, update *UpdateCourse) (*Course, error)
	DeleteCourseById(id int) error
}
