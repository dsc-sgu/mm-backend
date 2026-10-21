package checkpoints

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

// Checkpoint records one publish of a course: the full map of the block
// versions it made visible lives in checkpoint_blocks.
type Checkpoint struct {
	ID        uuid.UUID `json:"id"        db:"id"         binding:"required"`
	CourseID  uuid.UUID `json:"courseID"  db:"course_id"  binding:"required"`
	CreatedBy uuid.UUID `json:"createdBy" db:"created_by" binding:"required"`
	CreatedAt time.Time `json:"createdAt" db:"created_at" binding:"required"`
}

var (
	ErrNothingToPublish   = errors.New("the user has no draft to publish")
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	ErrTaskGroupNameTaken = errors.New(
		"a restored task group has the name of an existing one",
	)
)

// ConflictError is returned by a publish when a block was changed by the
// author and by another publish since the author's base version in a way
// that cannot be merged. Nothing is published.
type ConflictError struct {
	BlockIDs []uuid.UUID
}

func (e *ConflictError) Error() string {
	ids := make([]string, len(e.BlockIDs))
	for i, id := range e.BlockIDs {
		ids[i] = id.String()
	}
	return fmt.Sprintf(
		"blocks changed by another publish since the draft was started: %s",
		strings.Join(ids, ", "),
	)
}

type Repo interface {
	// PublishDrafts turns every draft of userID in the course into a
	// published version, merging each with what was published since its
	// base version, and records a checkpoint. It fails with *ConflictError
	// if any draft cannot be merged, and with ErrNothingToPublish if the
	// user has no draft.
	PublishDrafts(
		ctx context.Context,
		courseID, userID uuid.UUID,
	) (*Checkpoint, error)
	// DiscardDrafts deletes every draft of userID in the course.
	DiscardDrafts(ctx context.Context, courseID, userID uuid.UUID) error
	GetCheckpoints(ctx context.Context, courseID uuid.UUID) ([]*Checkpoint, error)
	// GetCheckpointBlocks lists the blocks a checkpoint of the course made
	// visible, in order.
	GetCheckpointBlocks(
		ctx context.Context,
		courseID, checkpointID uuid.UUID,
	) ([]*blocks.Block, error)
	// RestoreCheckpoint publishes the course as it was at the checkpoint:
	// new versions for the blocks that differ and deletions for the blocks
	// added since, recorded as a new checkpoint. Drafts are left alone.
	RestoreCheckpoint(
		ctx context.Context,
		courseID, checkpointID, userID uuid.UUID,
	) (*Checkpoint, error)
}
