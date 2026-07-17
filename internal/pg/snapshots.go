package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

const (
	createSnapshotSQL = `
		INSERT INTO course_snapshots (course_id, version, status, created_by, created_at)
		VALUES (:course_id, :version, :status, :created_by, :created_at)
		RETURNING id
	`

	getSnapshotByIDSQL = `
		SELECT id, course_id, version, status, created_by, created_at
		FROM course_snapshots
		WHERE id = $1 AND status != 'stale'
	`

	getDraftSnapshotSQL = `
		SELECT id, course_id, version, status, created_by, created_at
		FROM course_snapshots
		WHERE course_id = $1 AND created_by = $2 AND status = 'draft'
		LIMIT 1
	`

	getPublishedSnapshotsByCourseIDSQL = `
		SELECT id, course_id, version, status, created_by, created_at
		FROM course_snapshots
		WHERE course_id = $1 AND status = 'published'
		ORDER BY version DESC, created_at DESC
	`

	updateSnapshotStatusSQL = `
		UPDATE course_snapshots
		SET status = $1
		WHERE id = $2
	`

	deleteAllSnapshotsByCourseIDSQL = `
		UPDATE course_snapshots
		SET status = 'stale'
		WHERE course_id = $1 AND status != 'stale'
	`
)

func (r *PGRepo) CreateSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshot *snapshots.Snapshot,
) (*snapshots.Snapshot, error) {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", createSnapshotSQL))

	stmt, err := tx.PrepareNamedContext(ctx, createSnapshotSQL)
	if err != nil {
		return nil, fmt.Errorf("tx prepare named statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			zap.L().
				Error("failed to close statement", zap.Error(err))
		}
	}()

	var newID uuid.UUID
	err = stmt.GetContext(ctx, &newID, snapshot)
	if err != nil {
		return nil, fmt.Errorf("tx create snapshot: %w", err)
	}

	snapshot.ID = newID
	return snapshot, nil
}

func (r *PGRepo) GetSnapshotByID(
	ctx context.Context,
	id uuid.UUID,
) (*snapshots.Snapshot, error) {
	zap.L().Debug("Executing query", zap.String("query", getSnapshotByIDSQL))

	var snapshot snapshots.Snapshot
	err := r.db.GetContext(ctx, &snapshot, getSnapshotByIDSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get snapshot by id: %w", err)
	}
	return &snapshot, nil
}

// GetPublishedSnapshotsByCourseID returns a list of all published snapshots for a course editing timeline
func (r *PGRepo) GetPublishedSnapshotsByCourseID(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*snapshots.Snapshot, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getPublishedSnapshotsByCourseIDSQL))

	if courseID == uuid.Nil {
		return nil, fmt.Errorf("get published snapshots: course id is nil")
	}

	snapshotsList := make([]*snapshots.Snapshot, 0)

	err := r.db.SelectContext(
		ctx,
		&snapshotsList,
		getPublishedSnapshotsByCourseIDSQL,
		courseID,
	)
	if err != nil {
		return nil, fmt.Errorf("get published snapshots by course id: %w", err)
	}

	return snapshotsList, nil
}

// FindUserDraft returns unfinished draft snapshot for user
func (r *PGRepo) FindUserDraft(
	ctx context.Context,
	courseID, userID uuid.UUID,
) (*snapshots.Snapshot, error) {
	zap.L().Debug("Executing query", zap.String("query", getDraftSnapshotSQL))

	var snapshot snapshots.Snapshot
	err := r.db.GetContext(
		ctx,
		&snapshot,
		getDraftSnapshotSQL,
		courseID,
		userID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user draft: %w", err)
	}
	return &snapshot, nil
}

// UpdateSnapshotStatus updates a snapshot's status within an existing transaction.
func (r *PGRepo) UpdateSnapshotStatus(
	ctx context.Context,
	tx *sqlx.Tx,
	id uuid.UUID,
	status snapshots.Status,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", updateSnapshotStatusSQL))

	res, err := tx.ExecContext(ctx, updateSnapshotStatusSQL, status, id)
	if err != nil {
		return fmt.Errorf("tx update snapshot status: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CreateDraftFromActual atomically creates a draft from the actual snapshot
// and copies all its blocks into the draft.
func (r *PGRepo) CreateDraftFromActual(
	ctx context.Context,
	courseID uuid.UUID,
	targetVersion int,
	userID uuid.UUID,
	actualSnapshotID uuid.UUID,
) (*snapshots.Snapshot, error) {
	var createdDraft *snapshots.Snapshot

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		draft := &snapshots.Snapshot{
			CourseID:  courseID,
			Version:   targetVersion,
			Status:    snapshots.DraftStatus,
			CreatedBy: userID,
			CreatedAt: time.Now(),
		}

		var txErr error
		createdDraft, txErr = r.CreateSnapshot(ctx, tx, draft)
		if txErr != nil {
			return txErr
		}

		return r.CopyBlocksToSnapshot(
			ctx,
			tx,
			actualSnapshotID,
			createdDraft.ID,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("tx copy blocks to new snapshot: %w", err)
	}

	return createdDraft, nil
}

// SwitchSnapshotContent atomically deletes the current draft's blocks and
// copies blocks from the target snapshot.
func (r *PGRepo) SwitchSnapshotContent(
	ctx context.Context,
	draftSnapshotID uuid.UUID,
	targetSnapshotID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		zap.L().
			Debug("Executing block delete query within transaction", zap.String("query", deleteAllBlocksBySnapshotIDSQL))

		// Delete current draft blocks
		if err := r.DeleteAllBlocksBySnapshotID(
			ctx,
			tx,
			draftSnapshotID,
		); err != nil {
			return fmt.Errorf(
				"tx delete current draft blocks: %w",
				err,
			)
		}

		zap.L().
			Debug("Executing block copying query within transaction", zap.String("query", copyBlocksToSnapshotSQL))

		// Copy blocks from target
		if err := r.CopyBlocksToSnapshot(
			ctx,
			tx,
			targetSnapshotID,
			draftSnapshotID,
		); err != nil {
			return fmt.Errorf("tx copy blocks from target to draft: %w", err)
		}

		return nil
	})
}

// DeleteAllSnapshotsByCourseID marks all snapshots of a course as 'stale'. It is used for a cascading soft deletion of a course.
func (r *PGRepo) DeleteAllSnapshotsByCourseID(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", deleteAllSnapshotsByCourseIDSQL))

	if courseID == uuid.Nil {
		return fmt.Errorf("snapshots cascade delete: course id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteAllSnapshotsByCourseIDSQL, courseID)
	if err != nil {
		return fmt.Errorf("tx soft delete snapshots: %w", err)
	}
	return nil
}

// DiscardDraft atomically marks the draft snapshot as 'stale' and deletes all its blocks.
func (r *PGRepo) DiscardDraft(
	ctx context.Context,
	draftSnapshotID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.UpdateSnapshotStatus(
			ctx,
			tx,
			draftSnapshotID,
			snapshots.StaleStatus,
		); err != nil {
			return fmt.Errorf("discard draft update status: %w", err)
		}

		if err := r.DeleteAllBlocksBySnapshotID(
			ctx,
			tx,
			draftSnapshotID,
		); err != nil {
			return fmt.Errorf("discard draft deleting blocks: %w", err)
		}

		return nil
	})
}
