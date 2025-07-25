package disciplines

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

func NewPGRepo(db *sqlx.DB, logger *zap.Logger) Repo {
	return &PGRepo{db, logger}
}

const createDisciplineSql = `
	INSERT INTO discipline (id, name)
	VALUES (:id, :name)
	RETURNING id
`

func (r *PGRepo) Create(model *CreateDisciplineType) (*DisciplineType, error) {

	r.logger.Debug("Executing query", zap.String("query", createDisciplineSql))

	newDiscipline := DisciplineType{
		Name: model.Name,
		DisciplineID: DisciplineID{
			ID: model.ID,
		},
	}

	rows, err := r.db.NamedQuery(createDisciplineSql, newDiscipline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var returnedId uuid.UUID
	if rows.Next() {
		if err := rows.Scan(&returnedId); err != nil {
			return nil, err
		}
	}

	return &newDiscipline, nil
}

const getByIdSql = `
    SELECT id, name
    FROM discipline
    WHERE id = $1
`

func (r *PGRepo) GetById(id uuid.UUID) (*DisciplineType, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var discipline DisciplineType
	err := r.db.GetContext(context.Background(), &discipline, getByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &discipline, nil
}

const updateByIdSql = `
    UPDATE discipline
    SET name = $1
    WHERE id = $2
    RETURNING id, name
`

func (r *PGRepo) UpdateById(id uuid.UUID, model *CreateDisciplineType) (*DisciplineType, error) {
	r.logger.Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRow(
		updateByIdSql,
		model.Name,
		id,
	)

	var discipline DisciplineType

	err := row.Scan(
		&discipline.ID,
		&discipline.Name,
	)

	if err != nil {
		return nil, err
	}

	return &discipline, nil
}

const deleteByIdSql = `
    DELETE FROM discipline
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
