package blocks

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateBlock(ctx context.Context, model *CreateBlock) (*Block, error)
	GetBlockById(ctx context.Context, id uuid.UUID) (*Block, error)
	GetAllBlocksByCourseId(courseId uuid.UUID) ([]*Block, error)
	UpdateBlockById(id uuid.UUID, update *UpdateBlock) (*Block, error)
	UnlinkBlockById(courseID, blockID uuid.UUID) (*Block, error)
	DeleteBlockById(id uuid.UUID) error
}
