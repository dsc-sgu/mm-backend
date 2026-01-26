package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

const (
	createBlockSQL = `
		INSERT INTO block (block_type, data, course_id, position)
		VALUES (:block_type, :data, :course_id, :position)
		RETURNING id
	`

	nextPositionSQL = `
		SELECT COALESCE(MAX(position), 0)
		FROM block
		WHERE course_id = $1
	`

	getBlockByIdSQL = `
		SELECT id, block_type, data, course_id, position
		FROM block
		WHERE id = $1
	`

	getAllBlocksByCourseIdSQL = `
		SELECT *
		FROM block
		WHERE course_id = $1
	`

	updateBlockByIdSQL = `
		UPDATE block
		SET course_id = $1, data = $2, position = $3
		WHERE id = $4
		RETURNING id, block_type, data, course_id, position
	`

	UnlinkByIdSQL = `
		UPDATE block
		SET course_id = NULL
		WHERE course_id = $1 AND id = $2
		RETURNING id, block_type, data, course_id, position
	`

	deleteBlockByIdSQL = `
		DELETE FROM block
		WHERE id = $1
	`
)

func (r *PGRepo) CreateBlock(
	ctx context.Context,
	RequestBlock *blocks.CreateBlock,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", nextPositionSQL))

	position := 0

	err := r.db.GetContext(
		ctx,
		&position,
		nextPositionSQL,
		RequestBlock.CourseID,
	)
	if err != nil {
		return nil, fmt.Errorf("create block: scan next position: %w", err)
	}

	zap.L().Debug("Executing query", zap.String("query", createBlockSQL))

	newBlock := blocks.Block{
		BlockType: RequestBlock.BlockType,
		Data:      RequestBlock.Data,
		CourseID:  RequestBlock.CourseID,
		Position:  position + 1,
	}

	rows, err := r.db.NamedQuery(createBlockSQL, newBlock)
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

func (r *PGRepo) GetAllBlocksByCourseID(id uuid.UUID) ([]*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockByIdSQL))

	var blockList []*blocks.Block
	rows, err := r.db.Queryx(getAllBlocksByCourseIdSQL, id)
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

func (r *PGRepo) UpdateBlockByID(
	id uuid.UUID,
	update *blocks.UpdateBlock,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", updateBlockByIdSQL))

	row := r.db.QueryRowx(
		updateBlockByIdSQL,
		update.CourseID,
		update.Data,
		update.Position,
		id,
	)

	var block blocks.Block

	err := row.StructScan(&block)
	if err != nil {
		return nil, err
	}

	return &block, nil
}

func (r *PGRepo) UnlinkBlockByID(
	courseID uuid.UUID,
	blockID uuid.UUID,
) (*blocks.Block, error) {
	zap.L().
		Debug("Executing query", zap.String("query", UnlinkByIdSQL))

	row := r.db.QueryRowx(
		UnlinkByIdSQL,
		courseID,
		blockID,
	)

	var unlinkedBlock blocks.Block

	err := row.StructScan(&unlinkedBlock)
	if err != nil {
		return nil, err
	}

	return &unlinkedBlock, nil
}

func (r *PGRepo) DeleteBlockByID(id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteBlockByIdSQL))

	res, err := r.db.Exec(deleteBlockByIdSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
