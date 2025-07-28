package blocks

import (
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateBlock) (*Block, error)
	GetById(id uuid.UUID) (*Block, error)
	GetAllBlocksByCourseId(courseId uuid.UUID) ([]*Block, error)
	UpdateById(id uuid.UUID, update *UpdateBlock) (*Block, error)
	UnlinkFromCourseById(courseID, blockID uuid.UUID) (*Block, error)
	DeleteById(id uuid.UUID) error
}
