package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
)

const (
	getCourseLockSQL = `
		SELECT course_id, user_id, session_id, expires_at, (expires_at > NOW()) AS is_valid
		FROM course_locks
		WHERE course_id = $1
	`

	setCourseLockSQL = `
		INSERT INTO course_locks (course_id, user_id, session_id, expires_at)
		VALUES ($1, $2, $3, NOW() + MAKE_INTERVAL(secs => $4))
		ON CONFLICT (course_id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    session_id = EXCLUDED.session_id,
		    expires_at = EXCLUDED.expires_at
		WHERE course_locks.expires_at < NOW()
		   OR (course_locks.user_id = EXCLUDED.user_id AND course_locks.session_id = EXCLUDED.session_id)
		RETURNING course_id, user_id, session_id, expires_at
	`

	refreshCourseLockSQL = `
		UPDATE course_locks
		SET expires_at = NOW() + MAKE_INTERVAL(secs => $1)
		WHERE course_id = $2 AND user_id = $3 AND session_id = $4 AND expires_at > NOW()
	`

	deleteCourseLockSQL = `
		DELETE FROM course_locks
		WHERE course_id = $1 AND user_id = $2 AND session_id = $3
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

// SetLock creates a new course lock for a user, or takes over an existing one
// if it has expired or already belongs to the same user/session
func (r *PGRepo) SetLock(
	ctx context.Context,
	model *locks.LockSession,
	ttlSeconds int,
) (*locks.Lock, error) {
	zap.L().Debug("Executing query", zap.String("query", setCourseLockSQL))

	var newLock locks.Lock
	err := r.db.QueryRowxContext(
		ctx,
		setCourseLockSQL,
		model.CourseID,
		model.UserID,
		model.SessionID,
		ttlSeconds,
	).StructScan(&newLock)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, locks.ErrLockHeldByAnother
		}
		return nil, fmt.Errorf("set lock: %w", err)
	}

	return &newLock, nil
}

// RefreshLock refreshes the lifetime of a course lock
func (r *PGRepo) RefreshLock(
	ctx context.Context,
	model *locks.LockSession,
	ttlSeconds int,
) error {
	zap.L().Debug("Executing query", zap.String("query", refreshCourseLockSQL))

	res, err := r.db.ExecContext(
		ctx,
		refreshCourseLockSQL,
		ttlSeconds,
		model.CourseID,
		model.UserID,
		model.SessionID,
	)
	if err != nil {
		return fmt.Errorf("refresh lock: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("refresh lock rows affected: %w", err)
	}
	if affected > 0 {
		return nil
	}

	return r.classifyLockFailure(ctx, model)
}

// classifyLockFailure re-reads the current lock state to turn a failed
// guarded write into a precise sentinel error.
func (r *PGRepo) classifyLockFailure(
	ctx context.Context,
	model *locks.LockSession,
) error {
	currentLock, err := r.GetLock(ctx, model.CourseID)
	if err != nil {
		return fmt.Errorf("classify lock failure: %w", err)
	}
	if currentLock == nil {
		return locks.ErrLockNotFound
	}
	if currentLock.UserID != model.UserID ||
		currentLock.SessionID != model.SessionID {
		return locks.ErrLockHeldByAnother
	}
	return locks.ErrLockExpired
}

func (r *PGRepo) Unlock(ctx context.Context, model *locks.LockSession) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseLockSQL))

	_, err := r.db.ExecContext(
		ctx,
		deleteCourseLockSQL,
		model.CourseID,
		model.UserID,
		model.SessionID,
	)
	if err != nil {
		return fmt.Errorf("delete course lock: %w", err)
	}

	return nil
}
