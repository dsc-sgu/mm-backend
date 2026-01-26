package blocks

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateBlock(ctx context.Context, model *CreateBlock) (*Block, error)
	GetBlockByID(ctx context.Context, id uuid.UUID) (*Block, error)
	GetAllBlocksByCourseID(courseID uuid.UUID) ([]*Block, error)
	UpdateBlockByID(id uuid.UUID, update *UpdateBlock) (*Block, error)
	UnlinkBlockByID(courseID, blockID uuid.UUID) (*Block, error)
	DeleteBlockByID(id uuid.UUID) error
}
