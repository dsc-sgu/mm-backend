package blocks

import (
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/google/uuid"
)

type Repo interface {
	Create(model *dto.CreateBlockType, courseId uuid.UUID) (*dto.BlockType, error)
	GetById(id uuid.UUID) (*dto.BlockType, error)
	UpdateById(id uuid.UUID, update *dto.UpdateBlockType) (*dto.BlockType, error)
	DeleteById(id uuid.UUID) error
}
