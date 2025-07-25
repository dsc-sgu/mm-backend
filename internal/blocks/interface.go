package blocks

import (
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateBlockType, courseId uuid.UUID) (*BlockType, error)
	GetById(id uuid.UUID) (*BlockType, error)
	GetAllBlocksByCourseId(courseId uuid.UUID) ([]*BlockType, error)
	UpdateById(id uuid.UUID, update *UpdateBlockType) (*BlockType, error)
	UnlinkFromCourseById(id uuid.UUID) (*BlockType, error)
	DeleteById(id uuid.UUID) error
}
