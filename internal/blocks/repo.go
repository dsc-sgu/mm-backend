package blocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateBlockCommand is the input for creating a block. If BlockType is
// "task", Task must be set, since a task is just a block subtype rather
// than a separate entity - creating one always creates the other.
type CreateBlockCommand struct {
	CourseID     uuid.UUID       `json:"-"`
	BlockType    string          `json:"blockType"`
	Data         json.RawMessage `json:"data"`
	AfterBlockID *uuid.UUID      `json:"afterBlockID,omitempty"`
	Task         *TaskData       `json:"task,omitempty"`
	Actor        EditContext     `json:"-"`
}

// TaskData is the input (on create) and output (on read/patch) shape for a
// task-type block's task-specific fields.
type TaskData struct {
	TaskGroupID uuid.UUID  `json:"taskGroupId"`
	Name        string     `json:"name"`
	Patterns    []string   `json:"patterns,omitempty"`
	MaxGrade    float32    `json:"maxGrade"`
	MaxAttempts int        `json:"maxAttempts"`
	AvailableAt *time.Time `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time `json:"deadlineAt,omitempty"`
}

// TaskUpdate is the input for patching a task-type block's task-specific
// fields. TaskGroupID and Name are immutable after creation, so they have no
// place here.
type TaskUpdate struct {
	Patterns    *[]string  `json:"patterns,omitempty"`
	MaxGrade    *float32   `json:"maxGrade,omitempty"`
	MaxAttempts *int       `json:"maxAttempts,omitempty"`
	AvailableAt *time.Time `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time `json:"deadlineAt,omitempty"`
}

// PatchBlockCommand is the input for patching a block's own data and,
// if it is a task-type block, its task fields, in one request.
type PatchBlockCommand struct {
	CourseID   uuid.UUID       `json:"-"`
	SnapshotID uuid.UUID       `json:"-"`
	BlockID    uuid.UUID       `json:"-"`
	BlockType  *string         `json:"blockType,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"      swaggertype:"object"`
	Task       *TaskUpdate     `json:"task,omitempty"`
	Actor      EditContext     `json:"-"`
}

// CreatedBlock is the result of creating a block.
type CreatedBlock struct {
	BlockID        uuid.UUID
	TaskID         *uuid.UUID
	SnapshotID     uuid.UUID
	PositionLength int
}

// PatchedBlock is the result of patching a block. Task is populated whenever
// the block is (still) a task-type block, reflecting its current task data
// regardless of whether this particular patch touched it.
type PatchedBlock struct {
	Block *Block
	Task  *TaskData
}

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

// EditContext identifies the actor and editing session performing an operation.
type EditContext struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

// BlockRef identifies a specific block within its course and snapshot.
type BlockRef struct {
	BlockID    uuid.UUID
	CourseID   uuid.UUID
	SnapshotID uuid.UUID
}

type Repo interface {
	CreateBlock(
		ctx context.Context,
		command CreateBlockCommand,
	) (*CreatedBlock, error)
	PatchBlock(
		ctx context.Context,
		command PatchBlockCommand,
	) (*PatchedBlock, error)
	GetBlockByID(
		ctx context.Context,
		editCtx EditContext,
		ref BlockRef,
	) (*Block, error)
	GetAllBlocksBySnapshotID(
		ctx context.Context,
		snapshotID uuid.UUID,
	) ([]*Block, error)
	MoveBlock(
		ctx context.Context,
		editCtx EditContext,
		ref BlockRef,
		afterBlockID *uuid.UUID,
	) (string, error)
	DeleteBlockByID(
		ctx context.Context,
		editCtx EditContext,
		ref BlockRef,
	) error
	RebalanceBlockPositions(ctx context.Context, snapshotID uuid.UUID) error
}
