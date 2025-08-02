package blocks

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type PGRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

var _ Repo = (*PGRepo)(nil)

func NewPGRepo(db *sqlx.DB, logger *zap.Logger) Repo {
	return &PGRepo{db, logger}
}

const createBlockSql = `
    INSERT INTO block (id, block_type, data, course_id, position)
    VALUES (:id, :block_type, :data, :course_id, :position)
    RETURNING id
`

const nextPositionSql = `
    SELECT COALESCE(MAX(position), 0)
	FROM block
	WHERE course_id = $1
`

func (r *PGRepo) Create(ctx context.Context, RequestBlock *CreateBlock) (*Block, error) {
	r.logger.Debug("Executing query", zap.String("query", nextPositionSql))

	position := 0

	err := r.db.GetContext(ctx, &position, nextPositionSql, RequestBlock.CourseId)
	if err != nil {
		return nil, err
	}

	r.logger.Debug("Executing query", zap.String("query", createBlockSql))

	newBlock := Block{
		Id:        uuid.New(),
		BlockType: RequestBlock.BlockType,
		Data:      RequestBlock.Data,
		CourseId:  RequestBlock.CourseId,
		Position:  position + 1,
	}

	rows, err := r.db.NamedQuery(createBlockSql, newBlock)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newBlock.Id); err != nil {
			return nil, err
		}
	}

	return &newBlock, nil
}

const getByIdSql = `
    SELECT id, block_type, data, course_id, position
    FROM block
    WHERE id = $1
`

func (r *PGRepo) GetById(ctx context.Context, id uuid.UUID) (*Block, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var block Block
	err := r.db.GetContext(ctx, &block, getByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

const getAllByCourseIdSql = `
    SELECT *
    FROM block
    WHERE course_id = $1
`

func (r *PGRepo) GetAllBlocksByCourseId(id uuid.UUID) ([]*Block, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var blockList []*Block
	rows, err := r.db.Query(getAllByCourseIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Error(err.Error())
		}
	}()

	for rows.Next() {
		var block Block
		if err := rows.Scan(
			&block.Id,
			&block.BlockType,
			&block.Data,
			&block.CourseId,
			&block.Position,
		); err != nil {
			return nil, err
		}
		blockList = append(blockList, &block)
	}
	if err = rows.Err(); err != nil {
		return blockList, err
	}
	return blockList, nil
}

const updateByIdSql = `
    UPDATE block
    SET course_id = $1, data = $2, position = $3
    WHERE id = $4
    RETURNING id, block_type, data, course_id, position
`

func (r *PGRepo) UpdateById(id uuid.UUID, update *UpdateBlock) (*Block, error) {
	r.logger.Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRow(
		updateByIdSql,
		update.CourseId,
		update.Data,
		update.Position,
		id,
	)

	var block Block

	err := row.Scan(
		&block.Id,
		&block.BlockType,
		&block.Data,
		&block.CourseId,
		&block.Position,
	)
	if err != nil {
		return nil, err
	}

	return &block, nil
}

const UnlinkFromCourseByIdSql = `
	UPDATE block
	SET course_id = NULL
	WHERE course_id = $1 AND id = $2
	RETURNING id, block_type, data, course_id, position
`

func (r *PGRepo) UnlinkFromCourseById(courseId uuid.UUID, blockId uuid.UUID) (*Block, error) {
	r.logger.Debug("Executing query", zap.String("query", UnlinkFromCourseByIdSql))

	row := r.db.QueryRowx(
		UnlinkFromCourseByIdSql,
		courseId,
		blockId,
	)

	var unlinkedBlock Block

	err := row.StructScan(&unlinkedBlock)

	if err != nil {
		return nil, err
	}

	return &unlinkedBlock, nil
}

const deleteByIdSql = `
    DELETE FROM block
    WHERE id = $1
`

func (r *PGRepo) DeleteById(id uuid.UUID) error {
	r.logger.Debug("Executing query", zap.String("query", deleteByIdSql))

	res, err := r.db.Exec(deleteByIdSql, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
