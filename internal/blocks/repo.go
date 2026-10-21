package blocks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CreateBlockCommand is the input for creating a block. Data is the content
// of the block and must be empty for a task block. If BlockType is "task",
// Task must be set: creating a task block creates its task.
type CreateBlockCommand struct {
	CourseID     uuid.UUID       `json:"-"`
	BlockType    string          `json:"blockType"`
	Data         json.RawMessage `json:"data,omitempty"`
	AfterBlockID *uuid.UUID      `json:"afterBlockID,omitempty"`
	Task         *TaskData       `json:"task,omitempty"`
	Actor        EditContext     `json:"-"`
}

// TaskData is the input shape for the task created together with a
// task-type block.
type TaskData struct {
	TaskGroup   TaskGroupRef    `json:"taskGroup"`
	Name        string          `json:"name"`
	Statement   json.RawMessage `json:"statement,omitempty"`
	Patterns    []string        `json:"patterns,omitempty"`
	MaxGrade    float32         `json:"maxGrade"`
	MaxAttempts int             `json:"maxAttempts"`
	AvailableAt *time.Time      `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time      `json:"deadlineAt,omitempty"`
}

// TaskGroupRef picks the task group of a new task: an existing group by ID,
// or a new group created together with the task. Exactly one is set.
type TaskGroupRef struct {
	ID  *uuid.UUID    `json:"id,omitempty"`
	New *NewTaskGroup `json:"new,omitempty"`
}

type NewTaskGroup struct {
	Name string `json:"name"`
}

// PatchBlockCommand is the input for patching a block's own data. Version
// is the version the author loaded and edits from: their own draft, or a
// published version that becomes the draft's base. BlockType may only
// repeat the block's current type. A task's fields are patched through the
// task itself.
type PatchBlockCommand struct {
	CourseID  uuid.UUID       `json:"-"`
	BlockID   uuid.UUID       `json:"-"`
	Version   int             `json:"version"             minimum:"1"`
	BlockType *string         `json:"blockType,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"      swaggertype:"object"`
	Actor     EditContext     `json:"-"`
}

// CreatedBlock is the result of creating a block.
type CreatedBlock struct {
	BlockID        uuid.UUID
	TaskID         *uuid.UUID
	TaskGroupID    *uuid.UUID
	PositionLength int
}

// PatchedBlock is the result of patching a block.
type PatchedBlock struct {
	Block *Block
}

// Block is one version of a block. A published version (Draft = false) is
// never changed; an edit creates a draft version owned by CreatedBy, with
// BaseVersion naming the published version it was copied from (nil for a
// block that was never published). A deleted block ends with a version
// with Deleted = true. A task block's data is built on read from its link
// to the task: {"version": 1, "taskId": ...}.
type Block struct {
	ID          uuid.UUID       `json:"id"        db:"id"           binding:"required"`
	Version     int             `json:"version"   db:"version"      binding:"required"`
	CourseID    uuid.UUID       `json:"courseID"  db:"course_id"    binding:"required"`
	BlockType   string          `json:"blockType" db:"block_type"   binding:"required"`
	Data        json.RawMessage `json:"data"      db:"data"         binding:"required"`
	Position    string          `json:"position"  db:"position"     binding:"required"`
	Draft       bool            `json:"draft"     db:"draft"        binding:"required"`
	Deleted     bool            `json:"-"         db:"deleted"`
	CreatedBy   uuid.UUID       `json:"createdBy" db:"created_by"   binding:"required"`
	BaseVersion *int            `json:"-"         db:"base_version"`
}

// MoveBlock is the input for moving a block. Version is the version the
// author loaded and moves from, as in PatchBlockCommand.
type MoveBlock struct {
	Version      int        `json:"version"                minimum:"1"`
	AfterBlockID *uuid.UUID `json:"afterBlockID,omitempty"`
}

// AdjacentPositions holds the positions of the blocks surrounding an
// insertion or move point, empty string meaning there is no neighbor on
// that side.
type AdjacentPositions struct {
	Prev string
	Next string
}

// EditContext identifies the author of a draft.
type EditContext struct {
	UserID uuid.UUID
}

// BlockRef identifies a block within its course. Version is the version
// the author loaded and edits from; a read leaves it zero.
type BlockRef struct {
	BlockID  uuid.UUID
	CourseID uuid.UUID
	Version  int
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
	// GetBlockByID returns the version of the block viewerID sees: their
	// own draft if they have one, else the latest published version.
	GetBlockByID(
		ctx context.Context,
		viewerID uuid.UUID,
		ref BlockRef,
	) (*Block, error)
	// GetCourseBlocks lists the live blocks of a course in order, each in
	// the version viewerID sees. With viewerID = uuid.Nil it is the
	// published course.
	GetCourseBlocks(
		ctx context.Context,
		courseID, viewerID uuid.UUID,
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
	RebalanceBlockPositions(ctx context.Context, courseID uuid.UUID) error
}
