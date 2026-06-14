package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

const (
	createBlockSQL = `
		INSERT INTO blocks (snapshot_id, block_type, data, position, created_at)
		VALUES (:snapshot_id, :block_type, :data, :position, NOW())
		RETURNING id
	`

	getBlockByIdSQL = `
		SELECT id, snapshot_id, block_type, data, position, deleted_at
		FROM blocks
		WHERE id = $1
	`

	getAllBlocksBySnapshotIdSQL = `
		SELECT id, snapshot_id, block_type, data, position
		FROM blocks
		WHERE snapshot_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC
	`

	updateBlockContentSQL = `
		UPDATE blocks
		SET block_type = $1, data = $2
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, snapshot_id, block_type, data, position
	`

	updateBlockPositionSQL = `
		UPDATE blocks
		SET position = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	deleteBlockByIdSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	deleteBlocksByCourseIdSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE snapshot_id IN (
			SELECT id FROM course_snapshots WHERE course_id = $1
		) AND deleted_at IS NULL
	`

	getBlockPositionSQL = `
		SELECT position 
		FROM blocks 
		WHERE id = $1 AND deleted_at IS NULL
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
)

func (r *PGRepo) CreateBlock(
	ctx context.Context,
	model *blocks.CreateBlock,
	position string,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", createBlockSQL))

	newBlock := blocks.Block{
		SnapshotID: model.SnapshotID,
		BlockType:  model.BlockType,
		Data:       model.Data,
		Position:   position,
	}

	rows, err := r.db.NamedQueryContext(ctx, createBlockSQL, newBlock)
	if err != nil {
		return nil, fmt.Errorf("create block: insert in db: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newBlock.ID); err != nil {
			return nil, fmt.Errorf("create block: scan block id: %w", err)
		}
	}

	return &newBlock, nil
}

func (r *PGRepo) GetBlockByID(
	ctx context.Context,
	id uuid.UUID,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockByIdSQL))

	var block blocks.Block
	err := r.db.GetContext(ctx, &block, getBlockByIdSQL, id)
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
		Debug("Executing query", zap.String("query", getAllBlocksBySnapshotIdSQL))

	var blockList []*blocks.Block
	rows, err := r.db.QueryxContext(
		ctx,
		getAllBlocksBySnapshotIdSQL,
		snapshotID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
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

// Returns prev and next block positions to calculate new position between them
func (r *PGRepo) GetPositionsForMove(
	ctx context.Context,
	snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) (string, string, error) {
	var leftPos, rightPos string

	// If after_block_id is null, get first block position
	if afterBlockID == nil {
		zap.L().
			Debug("Executing query", zap.String("query", getFirstBlockPositionSQL))
		err := r.db.GetContext(
			ctx,
			&rightPos,
			getFirstBlockPositionSQL,
			snapshotID,
		)
		if err != nil && err != sql.ErrNoRows {
			return "", "", fmt.Errorf("get first block position: %w", err)
		}
		return "", rightPos, nil
	}

	// Else get prev and next block positions
	zap.L().Debug("Executing query", zap.String("query", getBlockPositionSQL))
	err := r.db.GetContext(
		ctx,
		&leftPos,
		getBlockPositionSQL,
		*afterBlockID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf(
				"after_block_id %s not found",
				*afterBlockID,
			)
		}
		return "", "", fmt.Errorf("get prev block position: %w", err)
	}

	zap.L().
		Debug("Executing query", zap.String("query", getNextBlockPositionSQL))
	err = r.db.GetContext(
		ctx,
		&rightPos,
		getNextBlockPositionSQL,
		snapshotID,
		leftPos,
	)
	if err != nil && err != sql.ErrNoRows {
		return "", "", fmt.Errorf("get next block position: %w", err)
	}

	return leftPos, rightPos, nil
}

func (r *PGRepo) UpdateBlockContent(
	ctx context.Context,
	id uuid.UUID,
	blockType string,
	data []byte,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", updateBlockContentSQL))

	var block blocks.Block
	err := r.db.QueryRowxContext(ctx, updateBlockContentSQL, blockType, data, id).
		StructScan(&block)
	if err != nil {
		return nil, fmt.Errorf("update block content: %w", err)
	}

	return &block, nil
}

func (r *PGRepo) UpdateBlockPosition(
	ctx context.Context,
	id uuid.UUID,
	newPosition string,
) error {
	zap.L().
		Debug("Executing query", zap.String("query", updateBlockPositionSQL))

	res, err := r.db.ExecContext(ctx, updateBlockPositionSQL, newPosition, id)
	if err != nil {
		return fmt.Errorf("update block position: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PGRepo) DeleteBlockByID(ctx context.Context, id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteBlockByIdSQL))

	res, err := r.db.ExecContext(ctx, deleteBlockByIdSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PGRepo) DeleteAllBlocksByCourseID(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
) error {
	zap.L().
		Debug("Executing blocks cascade delete within transaction", zap.String("query", deleteBlocksByCourseIdSQL))

	if courseID == uuid.Nil {
		return fmt.Errorf("blocks cascade delete: course id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteBlocksByCourseIdSQL, courseID)
	if err != nil {
		return fmt.Errorf("tx soft delete blocks: %w", err)
	}
	return nil
}
