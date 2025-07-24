package courses

import (
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateCourseType, ownerId uuid.UUID) (*CourseType, error)
	GetById(id uuid.UUID) (*CourseType, error)
	GetCourselistPage(limit int, offset int) ([]*CourseType, error)
	UpdateById(id uuid.UUID, update *UpdateCourseType) (*CourseType, error)
	DeleteById(id uuid.UUID) error
}
