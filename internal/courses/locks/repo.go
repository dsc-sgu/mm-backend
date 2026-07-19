package locks

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Lock is the database representation of a course lock
type Lock struct {
	CourseID  uuid.UUID `db:"course_id"`
	UserID    uuid.UUID `db:"user_id"`
	SessionID uuid.UUID `db:"session_id"`
	ExpiresAt time.Time `db:"expires_at"`
	IsValid   bool
}

// LockSession is the input for setting or refreshing a lock
type LockSession struct {
	CourseID  uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type Repo interface {
	GetLock(ctx context.Context, courseID uuid.UUID) (*Lock, error)
	SetLock(
		ctx context.Context,
		model *LockSession,
		ttlSeconds int,
	) (*Lock, *uuid.UUID, error)
	RefreshLock(
		ctx context.Context,
		model *LockSession,
		ttlSeconds int,
	) (*uuid.UUID, error)
	Unlock(ctx context.Context, model *LockSession) error
}
