package courses

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Course is the database representation of a course.
type Course struct {
	ID               uuid.UUID  `json:"id"                         db:"id"                 binding:"required"`
	DisciplineID     uuid.UUID  `json:"disciplineID,omitempty"     db:"discipline_id"`
	ActiveSnapshotID *uuid.UUID `json:"activeSnapshotID,omitempty" db:"active_snapshot_id"`
	OwnerID          uuid.UUID  `json:"ownerID"                    db:"owner_id"           binding:"required"`
	Name             string     `json:"name"                       db:"name"               binding:"required"`
	Version          int        `json:"version"                    db:"version"            binding:"required"`
	CreatedAt        time.Time  `json:"createdAt"                  db:"created_at"         binding:"required"`
	DeletedAt        *time.Time `json:"-"                          db:"deleted_at"`
}

// CreateCourse is the input for creating a course, used by both the service and repository layers.
type CreateCourse struct {
	DisciplineID uuid.UUID `json:"disciplineID,omitempty" db:"discipline_id"`
	Name         string    `json:"name"                   db:"name"          binding:"required"`
}

// UpdateCourse is the input for updating a course, used by both the service and repository layers.
type UpdateCourse struct {
	OwnerID *uuid.UUID `json:"ownerID,omitempty" db:"owner_id"`
	Name    *string    `json:"name,omitempty"    db:"name"`
}

type Repo interface {
	// CreateCourseWithInitialSnapshot atomically creates a course together with
	// its first published snapshot and links them.
	CreateCourseWithInitialSnapshot(
		ctx context.Context,
		model *CreateCourse,
		ownerID uuid.UUID,
	) (*Course, error)
	GetCourseByID(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(
		ctx context.Context,
		limit int,
		lastID uuid.UUID,
	) ([]Course, error)
	UpdateCourseByID(
		ctx context.Context,
		id uuid.UUID,
		update *UpdateCourse,
	) (*Course, error)
	DeleteCourseByID(ctx context.Context, id uuid.UUID) error
	PublishDraft(
		ctx context.Context,
		courseID, draftSnapshotID uuid.UUID,
		expectedVersion int,
		userID, sessionID uuid.UUID,
	) error
}
