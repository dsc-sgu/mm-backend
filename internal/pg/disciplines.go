package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

const (
	createDisciplineSql = `
		INSERT INTO discipline (name)
		VALUES (:name)
		RETURNING id
	`

	getDisciplineByIdSql = `
		SELECT id, name
		FROM discipline
		WHERE id = $1
	`

	updateDisciplineByIdSql = `
		UPDATE discipline
		SET name = $1
		WHERE id = $2
		RETURNING id, name
	`

	deleteDisciplineByIdSql = `
		DELETE FROM discipline
		WHERE id = $1
	`
)

func (r *PGRepo) CreateDiscipline(
	model *disciplines.CreateDiscipline,
) (*disciplines.Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", createDisciplineSql))

	newDiscipline := disciplines.Discipline{
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

	if rows.Next() {
		if err := rows.Scan(&newDiscipline.Id); err != nil {
			return nil, err
		}
	}

	return &newDiscipline, nil
}

func (r *PGRepo) GetDisciplineById(
	ctx context.Context,
	id uuid.UUID,
) (*disciplines.Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", getDisciplineByIdSql))

	var discipline disciplines.Discipline
	err := r.db.GetContext(ctx, &discipline, getDisciplineByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &discipline, nil
}

func (r *PGRepo) UpdateDisciplineById(
	id uuid.UUID,
	model *disciplines.PatchDiscipline,
) (*disciplines.Discipline, error) {
	zap.L().
		Debug("Executing query", zap.String("query", updateDisciplineByIdSql))

	row := r.db.QueryRowx(
		updateDisciplineByIdSql,
		model.Name,
		id,
	)

	var discipline disciplines.Discipline

	err := row.StructScan(&discipline)
	if err != nil {
		return nil, err
	}

	return &discipline, nil
}

func (r *PGRepo) DeleteDisciplineById(id uuid.UUID) error {
	zap.L().
		Debug("Executing query", zap.String("query", deleteDisciplineByIdSql))

	res, err := r.db.Exec(deleteDisciplineByIdSql, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
