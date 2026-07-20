package blocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	DeletedAt  *time.Time      `json:"-"          db:"deleted_at"`
}

// CreateBlock is the input for creating a block, used by both the service and repository layers.
type CreateBlock struct {
	SnapshotID   uuid.UUID       `json:"-"                      db:"snapshot_id" binding:"required"`
	BlockType    string          `json:"blockType"              db:"block_type"  binding:"required"`
	Data         json.RawMessage `json:"data"                   db:"data"        binding:"required" swaggertype:"object"`
	AfterBlockID *uuid.UUID      `json:"afterBlockID,omitempty"`
}

// UpdateBlock is the input for updating a block, used by both the service and repository layers.
type UpdateBlock struct {
	BlockType *string         `json:"blockType,omitempty" db:"block_type"`
	Data      json.RawMessage `json:"data,omitempty"      db:"data"       swaggertype:"object"`
}

// MoveBlock is the input for moving a block.
type MoveBlock struct {
	AfterBlockID *uuid.UUID `json:"afterBlockID,omitempty"`
}

// AdjacentPositions holds the positions of the blocks surrounding an
// insertion or move point, empty string meaning there is no neighbor on
// that side.
type AdjacentPositions struct {
	Prev string
	Next string
}

type Repo interface {
	CreateBlock(
		ctx context.Context,
		model *CreateBlock,
		userID, sessionID uuid.UUID,
	) (*Block, error)
	GetBlockByID(ctx context.Context, id uuid.UUID) (*Block, error)
	GetAllBlocksBySnapshotID(
		ctx context.Context,
		snapshotID uuid.UUID,
	) ([]*Block, error)
	MoveBlock(
		ctx context.Context,
		blockID, snapshotID uuid.UUID,
		afterBlockID *uuid.UUID,
		userID, sessionID uuid.UUID,
	) (string, error)
	UpdateBlockContent(
		ctx context.Context,
		id, snapshotID uuid.UUID,
		model *UpdateBlock,
		userID, sessionID uuid.UUID,
	) (*Block, error)
	DeleteBlockByID(
		ctx context.Context,
		id, snapshotID uuid.UUID,
		userID, sessionID uuid.UUID,
	) error
	RebalanceBlockPositions(ctx context.Context, snapshotID uuid.UUID) error
}
