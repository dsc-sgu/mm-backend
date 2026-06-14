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
	AfterBlockID uuid.NullUUID `json:"afterBlockID" binding:"required"`
}

// DeleteBlockFromCourse is the input for unlinking a block from a course, used by both the service and repository layers.
type DeleteBlockFromCourse struct {
	CourseID uuid.UUID `json:"courseID" db:"course_id" binding:"required"`
	ID       uuid.UUID `json:"id"       db:"id"        binding:"required"`
}

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
