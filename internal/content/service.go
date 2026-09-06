package content

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

type CreateBlockCommand struct {
	CourseID     uuid.UUID       `json:"-"`
	BlockType    string          `json:"blockType"`
	Data         json.RawMessage `json:"data"`
	AfterBlockID *uuid.UUID      `json:"afterBlockID,omitempty"`
	Task         *TaskData       `json:"task,omitempty"`
	Actor        EditContext     `json:"-"`
}

type TaskData struct {
	TaskGroupID uuid.UUID  `json:"taskGroupId"`
	Name        string     `json:"name"`
	Patterns    []string   `json:"patterns,omitempty"`
	MaxGrade    float32    `json:"maxGrade"`
	MaxAttempts int        `json:"maxAttempts"`
	AvailableAt *time.Time `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time `json:"deadlineAt,omitempty"`
}

// TaskUpdate is the input for patching a task block's task-specific fields.
// TaskGroupID and Name are immutable after creation, so they have no place here.
type TaskUpdate struct {
	Patterns    *[]string  `json:"patterns,omitempty"`
	MaxGrade    *float32   `json:"maxGrade,omitempty"`
	MaxAttempts *int       `json:"maxAttempts,omitempty"`
	AvailableAt *time.Time `json:"availableAt,omitempty"`
	DeadlineAt  *time.Time `json:"deadlineAt,omitempty"`
}

type PatchBlockCommand struct {
	CourseID   uuid.UUID       `json:"-"`
	SnapshotID uuid.UUID       `json:"-"`
	BlockID    uuid.UUID       `json:"-"`
	BlockType  *string         `json:"blockType,omitempty"`
	Data       json.RawMessage `json:"data,omitempty" swaggertype:"object"`
	Task       *TaskUpdate     `json:"task,omitempty"`
	Actor      EditContext     `json:"-"`
}

type EditContext struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type CreatedBlockContent struct {
	BlockID        uuid.UUID
	TaskID         *uuid.UUID
	SnapshotID     uuid.UUID
	PositionLength int
}

// PatchedBlockContent is the result of patching a block. Task is populated
// whenever the block is (still) a task-type block, reflecting its current
// task data regardless of whether this particular patch touched it.
type PatchedBlockContent struct {
	Block *blocks.Block
	Task  *TaskData
}

var (
	// ErrTaskGroupNotFound is returned when a task's TaskGroupID does not
	// resolve to a task group belonging to the block's own course.
	ErrTaskGroupNotFound = errors.New("task group not found")
	// ErrInvalidTaskBlock is returned when task data is supplied for a block
	// that is not a task-type block, or a task-type block is missing its
	// task data.
	ErrInvalidTaskBlock = errors.New("task data supplied for a non-task block")
)

type Repo interface {
	CreateBlockContent(context.Context, CreateBlockCommand) (*CreatedBlockContent, error)
	PatchBlockContent(context.Context, PatchBlockCommand) (*PatchedBlockContent, error)
}

// RebalanceNotifier is notified when a newly created block's position grows
// past the configured threshold, so it can be shrunk back down asynchronously.
type RebalanceNotifier interface {
	Enqueue(snapshotID uuid.UUID)
}

type Service struct {
	repo              Repo
	rebalanceWorker   RebalanceNotifier
	lexoRankThreshold int
}

func NewService(repo Repo, rebalanceWorker RebalanceNotifier, lexoRankThreshold int) *Service {
	return &Service{repo: repo, rebalanceWorker: rebalanceWorker, lexoRankThreshold: lexoRankThreshold}
}

func (s *Service) CreateBlock(ctx context.Context, command CreateBlockCommand) (*CreatedBlockContent, error) {
	result, err := s.repo.CreateBlockContent(ctx, command)
	if err != nil {
		return nil, err
	}

	if result.PositionLength > s.lexoRankThreshold {
		s.rebalanceWorker.Enqueue(result.SnapshotID)
	}

	return result, nil
}

func (s *Service) PatchBlock(ctx context.Context, command PatchBlockCommand) (*PatchedBlockContent, error) {
	return s.repo.PatchBlockContent(ctx, command)
}
