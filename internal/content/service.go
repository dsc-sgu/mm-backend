package content

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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

type Repo interface {
	CreateBlockContent(context.Context, CreateBlockCommand) (*CreatedBlockContent, error)
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
