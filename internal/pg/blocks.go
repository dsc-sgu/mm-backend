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
		INSERT INTO blocks (snapshot_id, block_type, data, position)
		VALUES (:snapshot_id, :block_type, :data, :position)
		RETURNING id
	`

	getBlockByIDSQL = `
		SELECT id, snapshot_id, block_type, data, position, deleted_at
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

// GetPositionsForMove returns prev and next block positions to calculate new position between them
func (r *PGRepo) GetPositionsForMove(
	ctx context.Context,
	snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) (blocks.AdjacentPositions, error) {
	// If after_block_id is null, get first block position
	if afterBlockID == nil {
		zap.L().
			Debug("Executing query", zap.String("query", getFirstBlockPositionSQL))
		var rightPos string
		err := r.db.GetContext(
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
	zap.L().Debug("Executing query", zap.String("query", getBlockPositionSQL))
	var leftPos string
	err := r.db.GetContext(
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

	zap.L().
		Debug("Executing query", zap.String("query", getNextBlockPositionSQL))
	var rightPos string
	err = r.db.GetContext(
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

func (r *PGRepo) UpdateBlockContent(
	ctx context.Context,
	id uuid.UUID,
	model *blocks.UpdateBlock,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", updateBlockContentSQL))

	// nil Data must reach the driver as SQL NULL (not an empty string), so
	// that COALESCE leaves the existing column value untouched.
	var data any
	if len(model.Data) > 0 {
		data = string(model.Data)
	}

	var block blocks.Block
	err := r.db.QueryRowxContext(
		ctx,
		updateBlockContentSQL,
		model.BlockType,
		data,
		id,
	).
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
	zap.L().Debug("Executing query", zap.String("query", deleteBlockByIDSQL))

	res, err := r.db.ExecContext(ctx, deleteBlockByIDSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
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
