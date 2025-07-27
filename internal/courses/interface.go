package courses

import (
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateCourse, ownerId uuid.UUID) (*Course, error)
	GetById(id uuid.UUID) (*Course, error)
	GetCourselistPage(limit int, offset int) ([]*Course, error)
	UpdateById(id uuid.UUID, update *UpdateCourse) (*Course, error)
	DeleteById(id uuid.UUID) error
}
