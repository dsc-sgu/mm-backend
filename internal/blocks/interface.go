package blocks

import (
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateBlockType) (*BlockType, error)
	GetById(id uuid.UUID) (*BlockType, error)
	GetAllBlocksByCourseId(courseId uuid.UUID) ([]*BlockType, error)
	UpdateById(update *UpdateBlockType) (*BlockType, error)
	UnlinkFromCourseById(courseID, blockID uuid.UUID) (*BlockType, error)
	DeleteById(id uuid.UUID) error
}
