package locks

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Lock is the database representation of a course lock
type Lock struct {
	CourseID  uuid.UUID `json:"courseId"  db:"course_id"  binding:"required"`
	UserID    uuid.UUID `json:"userId"    db:"user_id"    binding:"required"`
	SessionID uuid.UUID `json:"sessionId" db:"session_id" binding:"required"`
	ExpiresAt time.Time `json:"expiresAt" db:"expires_at" binding:"required"`
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
	) (*Lock, error)
	RefreshLock(ctx context.Context, model *LockSession, ttlSeconds int) error
	Unlock(ctx context.Context, model *LockSession) error
}
