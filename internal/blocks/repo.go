package blocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// TextData is the database representation of a text block's data.
type TextData struct {
	Format string `json:"format"`
	Text   string `json:"text"`
}

// QuizData is the database representation of a quiz block's data.
type QuizData struct {
	QuestionQuantity int      `json:""`
	Questions        []string `json:""`
	Answers          []string `json:""`
}

// Block is the database representation of a block.
type Block struct {
	ID         uuid.UUID       `json:"id"         db:"id"          binding:"required"`
	SnapshotID uuid.UUID       `json:"snapshotID" db:"snapshot_id" binding:"required"`
	BlockType  string          `json:"blockType"  db:"block_type"  binding:"required"`
	Data       json.RawMessage `json:"data"       db:"data"        binding:"required"`
	Position   string          `json:"position"   db:"position"    binding:"required"`
	DeletedAt  *time.Time      `json:"deletedAt"  db:"deleted_at"`
}

// CreateBlock is the input for creating a block, used by both the service and repository layers.
type CreateBlock struct {
	SnapshotID   uuid.UUID       `json:"snapshotID"   db:"snapshot_id" binding:"required"`
	BlockType    string          `json:"blockType"    db:"block_type"  binding:"required"`
	Data         json.RawMessage `json:"data"         db:"data"                           swaggertype:"object"`
	AfterBlockID *uuid.UUID      `json:"afterBlockID"`
}

// UpdateBlock is the input for updating a block, used by both the service and repository layers.
type UpdateBlock struct {
	BlockType string          `json:"blockType" db:"block_type"`
	Data      json.RawMessage `json:"data"      db:"data"       swaggertype:"object"`
}

// MoveBlock is the input for moving a block.
type MoveBlock struct {
	AfterBlockID *uuid.UUID `json:"afterBlockID" binding:"required"`
}

type Repo interface {
	CreateBlock(
		ctx context.Context,
		model *CreateBlock,
		position string,
	) (*Block, error)
	GetBlockByID(ctx context.Context, id uuid.UUID) (*Block, error)
	GetAllBlocksBySnapshotID(
		ctx context.Context,
		snapshotID uuid.UUID,
	) ([]*Block, error)
	GetPositionsForMove(
		ctx context.Context,
		snapshotID uuid.UUID,
		afterBlockID *uuid.UUID,
	) (string, string, error)
	UpdateBlockContent(
		ctx context.Context,
		id uuid.UUID,
		blockType string,
		data []byte,
	) (*Block, error)
	UpdateBlockPosition(
		ctx context.Context,
		id uuid.UUID,
		newPosition string,
	) error
	DeleteBlockByID(ctx context.Context, id uuid.UUID) error
	DeleteAllBlocksByCourseID(
		ctx context.Context,
		tx *sqlx.Tx,
		courseID uuid.UUID,
	) error
}
