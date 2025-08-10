package disciplines

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type PGRepo struct {
	db *sqlx.DB
}

func NewPGRepo(db *sqlx.DB) Repo {
	return &PGRepo{db}
}

const createDisciplineSql = `
	INSERT INTO discipline (id, name)
	VALUES (:id, :name)
	RETURNING id
`

func (r *PGRepo) Create(model *CreateDiscipline) (*Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", createDisciplineSql))

	u := uuid.New()

	newDiscipline := Discipline{
		Id:   u,
		Name: model.Name,
	}

	rows, err := r.db.NamedQuery(createDisciplineSql, newDiscipline)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

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

func (r *PGRepo) GetById(ctx context.Context, id uuid.UUID) (*Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", getByIdSql))

	var discipline Discipline
	err := r.db.GetContext(ctx, &discipline, getByIdSql, id)
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

func (r *PGRepo) UpdateById(id uuid.UUID, model *PatchDiscipline) (*Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRowx(
		updateByIdSql,
		model.Name,
		id,
	)

	var discipline Discipline

	err := row.StructScan(&discipline)
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
	zap.L().Debug("Executing query", zap.String("query", deleteByIdSql))

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
