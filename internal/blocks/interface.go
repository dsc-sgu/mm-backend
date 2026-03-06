package blocks

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateBlock(ctx context.Context, model *CreateBlock) (*Block, error)
	GetBlockByID(ctx context.Context, id uuid.UUID) (*Block, error)
	GetAllBlocksByCourseID(
		ctx context.Context,
		courseID uuid.UUID,
	) ([]*Block, error)
	UpdateBlockByID(
		ctx context.Context,
		id uuid.UUID,
		update *UpdateBlock,
	) (*Block, error)
	UnlinkBlockByID(
		ctx context.Context,
		courseID, blockID uuid.UUID,
	) (*Block, error)
	DeleteBlockByID(ctx context.Context, id uuid.UUID) error
}
