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
	CreateSnapshot(
		ctx context.Context,
		tx *sqlx.Tx,
		snapshot *Snapshot,
	) (*Snapshot, error)
	GetSnapshotByID(ctx context.Context, id uuid.UUID) (*Snapshot, error)
	FindUserDraft(
		ctx context.Context,
		courseID, userID uuid.UUID,
	) (*Snapshot, error)
	UpdateSnapshotStatus(
		ctx context.Context,
		tx *sqlx.Tx,
		id uuid.UUID,
		status Status,
	) error
	CreateDraftFromActual(
		ctx context.Context,
		tx *sqlx.Tx,
		courseID uuid.UUID,
		targetVersion int,
		userID uuid.UUID,
		actualSnapshotID uuid.UUID,
	) (*Snapshot, error)
	SwitchSnapshotContent(
		ctx context.Context,
		tx *sqlx.Tx,
		draftSnapshotID uuid.UUID,
		targetSnapshotID uuid.UUID,
	) error
	DeleteAllSnapshotsByCourseID(
		ctx context.Context,
		tx *sqlx.Tx,
		courseID uuid.UUID,
	) error
}
