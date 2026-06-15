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

	getSnapshotByIdSQL = `
		SELECT id, course_id, version, status, created_by, created_at
		FROM course_snapshots
		WHERE id = $1
	`

	getDraftSnapshotSQL = `
		SELECT id, course_id, version, status, created_by, created_at
		FROM course_snapshots
		WHERE course_id = $1 AND created_by = $2 AND status = 'draft'
		LIMIT 1
	`

	updateSnapshotStatusSQL = `
		UPDATE course_snapshots
		SET status = $1
		WHERE id = $2
	`

	deleteAllSnapshotsByCourseIdSQL = `
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
	defer stmt.Close()

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
	zap.L().Debug("Executing query", zap.String("query", getSnapshotByIdSQL))

	var snapshot snapshots.Snapshot
	err := r.db.GetContext(ctx, &snapshot, getSnapshotByIdSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get snapshot by id: %w", err)
	}
	return &snapshot, nil
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

// CreateDraftFromActual creates draft from actual snapshot and copies all blocks to it
func (r *PGRepo) CreateDraftFromActual(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
	targetVersion int,
	userID uuid.UUID,
	actualSnapshotID uuid.UUID,
) (*snapshots.Snapshot, error) {
	draft := &snapshots.Snapshot{
		CourseID:  courseID,
		Version:   targetVersion,
		Status:    snapshots.DraftStatus,
		CreatedBy: userID,
		CreatedAt: time.Now(),
	}

	createdDraft, err := r.CreateSnapshot(ctx, tx, draft)
	if err != nil {
		return nil, err
	}

	err = r.CopyBlocksToSnapshot(ctx, tx, actualSnapshotID, createdDraft.ID)
	if err != nil {
		return nil, fmt.Errorf("tx copy blocks to new snapshot: %w", err)
	}

	return createdDraft, nil
}

// SwitchSnapshotContent deletes current draft blocks and copies blocks from target snapshot
func (r *PGRepo) SwitchSnapshotContent(
	ctx context.Context,
	tx *sqlx.Tx,
	draftSnapshotID uuid.UUID,
	targetSnapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing block delete query within transaction", zap.String("query", deleteAllBlocksBySnapshotIdSQL))

	// Delete current draft blocks
	err := r.DeleteAllBlocksBySnapshotID(ctx, tx, draftSnapshotID)
	if err != nil {
		return fmt.Errorf(
			"tx delete current draft blocks: %w",
			err,
		)
	}

	zap.L().
		Debug("Executing block copying query within transaction", zap.String("query", copyBlocksToSnapshotSQL))

	// Copy blocks from target
	err = r.CopyBlocksToSnapshot(ctx, tx, targetSnapshotID, draftSnapshotID)
	if err != nil {
		return fmt.Errorf("tx copy blocks from target to draft: %w", err)
	}

	return nil
}

// DeleteAllSnapshotsByCourseID marks all snapshots of a course as 'stale'. It is used for a cascading soft deletion of a course.
func (r *PGRepo) DeleteAllSnapshotsByCourseID(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", deleteAllSnapshotsByCourseIdSQL))

	if courseID == uuid.Nil {
		return fmt.Errorf("snapshots cascade delete: course id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteAllSnapshotsByCourseIdSQL, courseID)
	if err != nil {
		return fmt.Errorf("tx soft delete snapshots: %w", err)
	}
	return nil
}

// DiscardDraft marks draft snapshot as 'stale' and deletes all it's blocks
func (r *PGRepo) DiscardDraft(
	ctx context.Context,
	tx *sqlx.Tx,
	draftSnapshotID uuid.UUID,
) error {
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
}
