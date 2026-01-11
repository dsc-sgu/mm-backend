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
	createBlockSql = `
		INSERT INTO block (block_type, data, course_id, position)
		VALUES (:block_type, :data, :course_id, :position)
		RETURNING id
	`

	nextPositionSql = `
		SELECT COALESCE(MAX(position), 0)
		FROM block
		WHERE course_id = $1
	`

	getBlockByIdSql = `
		SELECT id, block_type, data, course_id, position
		FROM block
		WHERE id = $1
	`

	getAllBlocksByCourseIdSql = `
		SELECT *
		FROM block
		WHERE course_id = $1
	`

	updateBlockByIdSql = `
		UPDATE block
		SET course_id = $1, data = $2, position = $3
		WHERE id = $4
		RETURNING id, block_type, data, course_id, position
	`

	UnlinkByIdSql = `
		UPDATE block
		SET course_id = NULL
		WHERE course_id = $1 AND id = $2
		RETURNING id, block_type, data, course_id, position
	`

	deleteBlockByIdSql = `
		DELETE FROM block
		WHERE id = $1
	`
)

func (r *PGRepo) CreateBlock(
	ctx context.Context,
	RequestBlock *blocks.CreateBlock,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", nextPositionSql))

	position := 0

	err := r.db.GetContext(
		ctx,
		&position,
		nextPositionSql,
		RequestBlock.CourseId,
	)
	if err != nil {
		return nil, fmt.Errorf("create block: scan next position: %w", err)
	}

	zap.L().Debug("Executing query", zap.String("query", createBlockSql))

	newBlock := blocks.Block{
		BlockType: RequestBlock.BlockType,
		Data:      RequestBlock.Data,
		CourseId:  RequestBlock.CourseId,
		Position:  position + 1,
	}

	rows, err := r.db.NamedQuery(createBlockSql, newBlock)
	if err != nil {
		return nil, fmt.Errorf("create block: insert in db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newBlock.Id); err != nil {
			return nil, fmt.Errorf("create block: scan block id: %w", err)
		}
	}

	return &newBlock, nil
}

func (r *PGRepo) GetBlockById(
	ctx context.Context,
	id uuid.UUID,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockByIdSql))

	var block blocks.Block
	err := r.db.GetContext(ctx, &block, getBlockByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

func (r *PGRepo) GetAllBlocksByCourseId(id uuid.UUID) ([]*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockByIdSql))

	var blockList []*blocks.Block
	rows, err := r.db.Queryx(getAllBlocksByCourseIdSql, id)
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

func (r *PGRepo) UpdateBlockById(
	id uuid.UUID,
	update *blocks.UpdateBlock,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", updateBlockByIdSql))

	row := r.db.QueryRowx(
		updateBlockByIdSql,
		update.CourseId,
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

func (r *PGRepo) UnlinkBlockById(
	courseId uuid.UUID,
	blockId uuid.UUID,
) (*blocks.Block, error) {
	zap.L().
		Debug("Executing query", zap.String("query", UnlinkByIdSql))

	row := r.db.QueryRowx(
		UnlinkByIdSql,
		courseId,
		blockId,
	)

	var unlinkedBlock blocks.Block

	err := row.StructScan(&unlinkedBlock)
	if err != nil {
		return nil, err
	}

	return &unlinkedBlock, nil
}

func (r *PGRepo) DeleteBlockById(id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteBlockByIdSql))

	res, err := r.db.Exec(deleteBlockByIdSql, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
