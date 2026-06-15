package locks

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// TODO: Move to config
// Duration of course lock prolongation
const LockDuration = 60 * time.Second

var (
	ErrLockHeldByAnother = errors.New(
		"course lock is held by another user or session",
	)
	ErrLockExpired  = errors.New("course lock has expired")
	ErrLockNotFound = errors.New("course lock not found")
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
	UpsertLock(ctx context.Context, model *LockSession) (*Lock, error)
	RefreshLock(ctx context.Context, model *LockSession) error
	Unlock(ctx context.Context, courseID uuid.UUID) error
}
