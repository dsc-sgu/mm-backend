package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

const (
	createBlockSQL = `
		INSERT INTO blocks (snapshot_id, block_type, data, position)
		VALUES (:snapshot_id, :block_type, :data, :position)
		RETURNING id
	`

	getBlockByIDSQL = `
		SELECT id, snapshot_id, block_type, data, position, deleted_at
		FROM blocks
		WHERE id = $1 AND deleted_at IS NULL
	`

	getBlockSnapshotIDSQL = `
		SELECT snapshot_id
		FROM blocks
		WHERE id = $1 AND deleted_at IS NULL
	`

	getAllBlocksBySnapshotIDSQL = `
		SELECT id, snapshot_id, block_type, data, position
		FROM blocks
		WHERE snapshot_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC
	`

	updateBlockContentSQL = `
		UPDATE blocks
		SET block_type = COALESCE($1, block_type), data = COALESCE($2, data)
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, snapshot_id, block_type, data, position
	`

	updateBlockPositionSQL = `
		UPDATE blocks
		SET position = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	deleteBlockByIDSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	deleteAllBlocksBySnapshotIDSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE snapshot_id = $1 AND deleted_at IS NULL
	`

	getBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE id = $1 AND snapshot_id = $2 AND deleted_at IS NULL
	`

	getNextBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE snapshot_id = $1
		  AND position > $2
		  AND deleted_at IS NULL
		ORDER BY position ASC
		LIMIT 1
	`

	getFirstBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE snapshot_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC
		LIMIT 1
	`

	copyBlocksToSnapshotSQL = `
		INSERT INTO blocks (snapshot_id, block_type, data, position)
		SELECT $1, block_type, data, position
		FROM blocks
		WHERE snapshot_id = $2 AND deleted_at IS NULL
	`

	lockSnapshotSQL = `
		SELECT course_id, status
		FROM course_snapshots
		WHERE id = $1
		FOR UPDATE
	`

	lockSnapshotForRebalanceSQL = `
		SELECT 1
		FROM course_snapshots
		WHERE id = $1
		FOR UPDATE
	`

	lockCourseLockSQL = `
		SELECT user_id, session_id, (expires_at > NOW()) AS is_valid
		FROM course_locks
		WHERE course_id = $1
		FOR UPDATE
	`
)

// validateEditableSnapshot locks the snapshot row and the course's lock row
// within tx and verifies the snapshot is a draft
// currently locked by user/session
func validateEditableSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
	userID, sessionID uuid.UUID,
) error {
	var snapshot snapshots.Snapshot
	err := tx.GetContext(ctx, &snapshot, lockSnapshotSQL, snapshotID)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrSnapshotNotFound
		}
		return fmt.Errorf("lock snapshot: %w", err)
	}
	if snapshot.Status != snapshots.DraftStatus {
		return blocks.ErrSnapshotNotDraft
	}

	var lock locks.Lock
	err = tx.GetContext(ctx, &lock, lockCourseLockSQL, snapshot.CourseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return locks.ErrLockNotFound
		}
		return fmt.Errorf("lock course lock: %w", err)
	}
	if lock.UserID != userID || lock.SessionID != sessionID {
		return locks.ErrLockHeldByAnother
	}
	if !lock.IsValid {
		return locks.ErrLockExpired
	}

	return nil
}

// blockBelongsToSnapshot verifies blockID is a valid block
// of the snapshot with snapshotID within tx
func blockBelongsToSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	blockID, snapshotID uuid.UUID,
) error {
	var actualSnapshotID uuid.UUID
	err := tx.GetContext(ctx, &actualSnapshotID, getBlockSnapshotIDSQL, blockID)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrBlockNotFound
		}
		return fmt.Errorf("get block: %w", err)
	}
	if actualSnapshotID != snapshotID {
		return blocks.ErrBlockNotFound
	}
	return nil
}

// getPositionsForMoveTx returns prev and next block positions to calculate a
// new position between them
func getPositionsForMoveTx(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) (blocks.AdjacentPositions, error) {
	// If after_block_id is null, get first block position
	if afterBlockID == nil {
		var rightPos string
		err := tx.GetContext(
			ctx,
			&rightPos,
			getFirstBlockPositionSQL,
			snapshotID,
		)
		if err != nil && err != sql.ErrNoRows {
			return blocks.AdjacentPositions{}, fmt.Errorf(
				"get first block position: %w",
				err,
			)
		}
		return blocks.AdjacentPositions{Next: rightPos}, nil
	}

	// Else get prev and next block positions
	var leftPos string
	err := tx.GetContext(
		ctx,
		&leftPos,
		getBlockPositionSQL,
		*afterBlockID,
		snapshotID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.AdjacentPositions{}, fmt.Errorf(
				"%w: %s",
				blocks.ErrAfterBlockNotFound,
				*afterBlockID,
			)
		}
		return blocks.AdjacentPositions{}, fmt.Errorf(
			"get prev block position: %w",
			err,
		)
	}

	var rightPos string
	err = tx.GetContext(
		ctx,
		&rightPos,
		getNextBlockPositionSQL,
		snapshotID,
		leftPos,
	)
	if err != nil && err != sql.ErrNoRows {
		return blocks.AdjacentPositions{}, fmt.Errorf(
			"get next block position: %w",
			err,
		)
	}

	return blocks.AdjacentPositions{Prev: leftPos, Next: rightPos}, nil
}

func (r *PGRepo) CreateBlock(
	ctx context.Context,
	model *blocks.CreateBlock,
	userID, sessionID uuid.UUID,
) (*blocks.Block, error) {
	var newBlock blocks.Block

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			model.SnapshotID,
			userID,
			sessionID,
		); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			model.SnapshotID,
			model.AfterBlockID,
		)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}

		newBlock = blocks.Block{
			SnapshotID: model.SnapshotID,
			BlockType:  model.BlockType,
			Data:       model.Data,
			Position: blocks.CalculateMiddlePosition(
				positions.Prev,
				positions.Next,
			),
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", createBlockSQL))

		stmt, err := tx.PrepareNamedContext(ctx, createBlockSQL)
		if err != nil {
			return fmt.Errorf("tx prepare named statement for block: %w", err)
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				zap.L().Error("failed to close statement", zap.Error(err))
			}
		}()

		if err := stmt.GetContext(ctx, &newBlock.ID, newBlock); err != nil {
			return fmt.Errorf("tx create block: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &newBlock, nil
}

func (r *PGRepo) GetBlockByID(
	ctx context.Context,
	id uuid.UUID,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockByIDSQL))

	var block blocks.Block
	err := r.db.GetContext(ctx, &block, getBlockByIDSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

func (r *PGRepo) GetAllBlocksBySnapshotID(
	ctx context.Context,
	snapshotID uuid.UUID,
) ([]*blocks.Block, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getAllBlocksBySnapshotIDSQL))

	var blockList []*blocks.Block
	rows, err := r.db.QueryxContext(
		ctx,
		getAllBlocksBySnapshotIDSQL,
		snapshotID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	for rows.Next() {
		var block blocks.Block
		if err := rows.StructScan(&block); err != nil {
			return nil, err
		}
		blockList = append(blockList, &block)
	}
	if err = rows.Err(); err != nil {
		return blockList, err
	}
	return blockList, nil
}

func (r *PGRepo) MoveBlock(
	ctx context.Context,
	blockID, snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
	userID, sessionID uuid.UUID,
) (string, error) {
	var newPosition string

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			snapshotID,
			userID,
			sessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(
			ctx,
			tx,
			blockID,
			snapshotID,
		); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			snapshotID,
			afterBlockID,
		)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}
		newPosition = blocks.CalculateMiddlePosition(
			positions.Prev,
			positions.Next,
		)

		zap.L().
			Debug("Executing query within transaction", zap.String("query", updateBlockPositionSQL))

		res, err := tx.ExecContext(
			ctx,
			updateBlockPositionSQL,
			newPosition,
			blockID,
		)
		if err != nil {
			return fmt.Errorf("tx update block position: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("tx update block position rows affected: %w", err)
		}
		if affected == 0 {
			return blocks.ErrBlockNotFound
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return newPosition, nil
}

func (r *PGRepo) UpdateBlockContent(
	ctx context.Context,
	id, snapshotID uuid.UUID,
	model *blocks.UpdateBlock,
	userID, sessionID uuid.UUID,
) (*blocks.Block, error) {
	var block blocks.Block

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			snapshotID,
			userID,
			sessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(ctx, tx, id, snapshotID); err != nil {
			return err
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", updateBlockContentSQL))

		// nil Data must reach the driver as SQL NULL (not an empty string),
		// so that COALESCE leaves the existing column value untouched
		var data any
		if len(model.Data) > 0 {
			data = string(model.Data)
		}

		err := tx.QueryRowxContext(
			ctx,
			updateBlockContentSQL,
			model.BlockType,
			data,
			id,
		).StructScan(&block)
		if err != nil {
			return fmt.Errorf("tx update block content: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &block, nil
}

// RebalanceBlockPositions recomputes and updates every block's position
// for a specific snapshot to distribute the blocks evenly across the alphabet
func (r *PGRepo) RebalanceBlockPositions(
	ctx context.Context,
	snapshotID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var dummy int
		err := tx.GetContext(
			ctx,
			&dummy,
			lockSnapshotForRebalanceSQL,
			snapshotID,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("lock snapshot: %w", err)
		}

		var blocksList []*blocks.Block
		if err := tx.SelectContext(
			ctx,
			&blocksList,
			getAllBlocksBySnapshotIDSQL,
			snapshotID,
		); err != nil {
			return fmt.Errorf("get blocks: %w", err)
		}

		// Redistribute positions: block i gets the position at fraction
		// (i+1)/(totalBlocks+1) of the alphabet range, leaving a gap at both
		// ends for future inserts.
		totalBlocks := len(blocksList)
		for i, block := range blocksList {
			// Add 1 to the value of totalBlocks to leave a gap at the end of the alphabet
			newPos := blocks.IndexToPosition(i+1, totalBlocks+1)
			if block.Position == newPos {
				continue
			}
			if _, err := tx.ExecContext(
				ctx,
				updateBlockPositionSQL,
				newPos,
				block.ID,
			); err != nil {
				return fmt.Errorf("update block position: %w", err)
			}
		}

		return nil
	})
}

func (r *PGRepo) DeleteBlockByID(
	ctx context.Context,
	id, snapshotID uuid.UUID,
	userID, sessionID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			snapshotID,
			userID,
			sessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(ctx, tx, id, snapshotID); err != nil {
			return err
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", deleteBlockByIDSQL))

		_, err := tx.ExecContext(ctx, deleteBlockByIDSQL, id)
		if err != nil {
			return fmt.Errorf("tx delete block: %w", err)
		}

		return nil
	})
}

func (r *PGRepo) DeleteAllBlocksBySnapshotID(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing blocks delete within transaction", zap.String("query", deleteAllBlocksBySnapshotIDSQL))

	if snapshotID == uuid.Nil {
		return fmt.Errorf("blocks delete: snapshot id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteAllBlocksBySnapshotIDSQL, snapshotID)
	if err != nil {
		return fmt.Errorf("tx soft delete blocks by snapshot: %w", err)
	}
	return nil
}

// CopyBlocksToSnapshot copies blocks from one snapshot to another
func (r *PGRepo) CopyBlocksToSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	sourceSnapshotID uuid.UUID,
	targetSnapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing block copying query within transaction", zap.String("query", copyBlocksToSnapshotSQL))

	_, err := tx.ExecContext(
		ctx,
		copyBlocksToSnapshotSQL,
		targetSnapshotID,
		sourceSnapshotID,
	)
	if err != nil {
		return fmt.Errorf("tx copy blocks: %w", err)
	}
	return nil
}
