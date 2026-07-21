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
	CourseID     uuid.UUID       `json:"-"`
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

// BlockRef identifies a specific block within its course and snapshot,
// together with the editing session performing the operation.
type BlockRef struct {
	BlockID    uuid.UUID
	CourseID   uuid.UUID
	SnapshotID uuid.UUID
	UserID     uuid.UUID
	SessionID  uuid.UUID
}

type Repo interface {
	CreateBlock(
		ctx context.Context,
		model *CreateBlock,
		userID, sessionID uuid.UUID,
	) (*Block, error)
	GetBlockByID(ctx context.Context, ref BlockRef) (*Block, error)
	GetAllBlocksBySnapshotID(
		ctx context.Context,
		snapshotID uuid.UUID,
	) ([]*Block, error)
	MoveBlock(
		ctx context.Context,
		ref BlockRef,
		afterBlockID *uuid.UUID,
	) (string, error)
	UpdateBlockContent(
		ctx context.Context,
		ref BlockRef,
		model *UpdateBlock,
	) (*Block, error)
	DeleteBlockByID(ctx context.Context, ref BlockRef) error
	RebalanceBlockPositions(ctx context.Context, snapshotID uuid.UUID) error
}
