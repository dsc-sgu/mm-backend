package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
)

const (
	getCourseLockSQL = `
		SELECT course_id, user_id, session_id, expires_at
		FROM course_locks
		WHERE course_id = $1
	`

	setCourseLockSQL = `
		INSERT INTO course_locks (course_id, user_id, session_id, expires_at)
		VALUES ($1, $2, $3, NOW() + $4 * INTERVAL '1 second')
		ON CONFLICT (course_id) DO UPDATE
		SET user_id = EXCLUDED.user_id, 
		    session_id = EXCLUDED.session_id, 
		    expires_at = EXCLUDED.expires_at
		RETURNING course_id, user_id, session_id, expires_at
	`

	getCourseLockForUpdateSQL = `
		SELECT course_id, user_id, session_id, expires_at 
		FROM course_locks 
		WHERE course_id = $1 
		FOR UPDATE
	`

	refreshCourseLockSQL = `
		UPDATE course_locks 
		SET expires_at = NOW() + $1 * INTERVAL '1 second' 
		WHERE course_id = $2
	`

	deleteCourseLockSQL = `
		DELETE FROM course_locks
		WHERE course_id = $1
	`
)

func (r *PGRepo) GetLock(
	ctx context.Context,
	courseID uuid.UUID,
) (*locks.Lock, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseLockSQL))

	var lock locks.Lock
	err := r.db.GetContext(ctx, &lock, getCourseLockSQL, courseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get course lock: %w", err)
	}
	return &lock, nil
}

// SetLock creates a new course lock for a user
// or intercepts an existing one if it has expired or belongs to the same user
func (r *PGRepo) SetLock(
	ctx context.Context,
	model *locks.LockSession,
) (*locks.Lock, error) {
	zap.L().Debug("Executing query", zap.String("query", setCourseLockSQL))

	var lock locks.Lock
	seconds := int64(locks.LockDuration.Seconds())

	err := r.db.QueryRowxContext(ctx, setCourseLockSQL, model.CourseID, model.UserID, model.SessionID, seconds).
		StructScan(&lock)
	if err != nil {
		return nil, fmt.Errorf("set course lock: %w", err)
	}

	return &lock, nil
}

// RefreshLock refreshes the lifetime of a course lock
func (r *PGRepo) RefreshLock(
	ctx context.Context,
	model *locks.LockSession,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("refresh lock transaction failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentLock locks.Lock

	// Get the current lock for update
	err = tx.GetContext(
		ctx,
		&currentLock,
		getCourseLockForUpdateSQL,
		model.CourseID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return locks.ErrLockNotFound
		}
		return fmt.Errorf("refresh lock check: %w", err)
	}

	// Check if the lock is held by someone else
	if currentLock.UserID != model.UserID ||
		currentLock.SessionID != model.SessionID {
		return locks.ErrLockHeldByAnother
	}

	// Check if the lock has expired
	if time.Now().After(currentLock.ExpiresAt) {
		return locks.ErrLockExpired
	}

	// If the lock is still valid, refresh it's lifetime
	seconds := int64(locks.LockDuration.Seconds())
	_, err = tx.ExecContext(ctx, refreshCourseLockSQL, seconds, model.CourseID)
	if err != nil {
		return fmt.Errorf("refresh lock update: %w", err)
	}

	return tx.Commit()
}

func (r *PGRepo) Unlock(ctx context.Context, courseID uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseLockSQL))

	_, err := r.db.ExecContext(ctx, deleteCourseLockSQL, courseID)
	if err != nil {
		return fmt.Errorf("delete course lock: %w", err)
	}
	return nil
}
