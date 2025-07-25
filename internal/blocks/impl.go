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
    SELECT position
    FROM block
    WHERE course_id = $1
	ORDER BY position ASC
`

func (r *PGRepo) Create(RequestBlock *CreateBlockType) (*BlockType, error) {

	r.logger.Debug("Executing query", zap.String("query", nextPositionSql))

	positions, err := r.db.Query(nextPositionSql, RequestBlock.ID)
	if err != nil {
		return nil, err
	}
	defer positions.Close()

	var position int = 0

	if positions.Next() {
		if err := positions.Scan(&position); err != nil {
			return nil, err
		}
	}

	r.logger.Debug("Executing query", zap.String("query", createBlockSql))

	newBlock := BlockType{
		Id:        uuid.New(),
		BlockType: RequestBlock.BlockType,
		Data:      RequestBlock.Data,
		CourseId:  RequestBlock.ID,
		Position:  position + 1,
	}

	rows, err := r.db.NamedQuery(createBlockSql, newBlock)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (r *PGRepo) GetById(id uuid.UUID) (*BlockType, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var block BlockType
	err := r.db.GetContext(context.Background(), &block, getByIdSql, id)
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

func (r *PGRepo) GetAllBlocksByCourseId(id uuid.UUID) ([]*BlockType, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var block BlockType
	var blockList []*BlockType
	rows, err := r.db.Query(getAllByCourseIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
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

func (r *PGRepo) UpdateById(update *UpdateBlockType) (*BlockType, error) {
	r.logger.Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRow(
		updateByIdSql,
		update.CourseID.ID,
		update.Data,
		update.Position,
		update.BlockID.ID,
	)

	var block BlockType

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
	SET course_id = $1
	WHERE id = $2
`

func (r *PGRepo) UnlinkFromCourseById(id uuid.UUID) (*BlockType, error) {
	r.logger.Debug("Executing query", zap.String("query", deleteByIdSql))

	row := r.db.QueryRow(
		UnlinkFromCourseByIdSql,
		nil,
		id,
	)

	var unlinkedBlock BlockType

	err := row.Scan(
		&unlinkedBlock.Id,
		&unlinkedBlock.BlockType,
		&unlinkedBlock.Data,
		&unlinkedBlock.CourseId,
		&unlinkedBlock.Position,
	)

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
