package snapshots

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Status string

const (
	DraftStatus     Status = "draft"
	PublishedStatus Status = "published"
	StaleStatus     Status = "stale"
)

// Snapshot is the database representation of a snapshot.
type Snapshot struct {
	ID        uuid.UUID `json:"id"        db:"id"         binding:"required"`
	CourseID  uuid.UUID `json:"courseID"  db:"course_id"  binding:"required"`
	Version   int       `json:"version"   db:"version"    binding:"required"`
	Status    Status    `json:"status"    db:"status"     binding:"required"`
	CreatedBy uuid.UUID `json:"createdBy" db:"created_by" binding:"required"`
	CreatedAt time.Time `json:"createdAt" db:"created_at" binding:"required"`
}

type Repo interface {
	GetSnapshotByID(ctx context.Context, id uuid.UUID) (*Snapshot, error)
	GetPublishedSnapshotsByCourseID(
		ctx context.Context,
		courseID uuid.UUID,
	) ([]*Snapshot, error)
	FindUserDraft(
		ctx context.Context,
		courseID, userID uuid.UUID,
	) (*Snapshot, error)
	// CreateDraftFromActual atomically creates a draft snapshot and copies
	// all blocks from the actual snapshot into it.
	CreateDraftFromActual(
		ctx context.Context,
		courseID uuid.UUID,
		targetVersion int,
		userID uuid.UUID,
		actualSnapshotID uuid.UUID,
	) (*Snapshot, error)
	// SwitchSnapshotContent atomically replaces the draft's blocks with the
	// target snapshot's blocks.
	SwitchSnapshotContent(
		ctx context.Context,
		draftSnapshotID uuid.UUID,
		targetSnapshotID uuid.UUID,
	) error
	DeleteAllSnapshotsByCourseID(
		ctx context.Context,
		tx *sqlx.Tx,
		courseID uuid.UUID,
	) error
	// DiscardDraft atomically marks the draft snapshot as stale and deletes
	// all its blocks.
	DiscardDraft(
		ctx context.Context,
		draftSnapshotID uuid.UUID,
	) error
}
