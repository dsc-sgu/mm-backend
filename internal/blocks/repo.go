package blocks

import (
	"context"
	"encoding/json"

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
	ID        uuid.UUID       `json:"id"        db:"id"         binding:"required"`
	BlockType string          `json:"blockType" db:"block_type" binding:"required"`
	Data      json.RawMessage `json:"data"      db:"data"       binding:"required"`
	CourseID  uuid.UUID       `json:"courseID"  db:"course_id"  binding:"required"`
	Position  int             `json:"position"  db:"position"   binding:"required"`
}

// CreateBlock is the input for creating a block, used by both the service and repository layers.
type CreateBlock struct {
	CourseID  uuid.UUID       `json:"courseID"  db:"course_id" binding:"required"`
	BlockType string          `json:"blockType"                binding:"required"`
	Data      json.RawMessage `json:"data"                     binding:"required" swaggertype:"object"`
}

// UpdateBlock is the input for updating a block, used by both the service and repository layers.
type UpdateBlock struct {
	CourseID uuid.UUID       `json:"courseID"           db:"course_id" binding:"required"`
	Data     json.RawMessage `json:"data,omitempty"                                       swaggertype:"object"`
	Position *int            `json:"position,omitempty"`
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
