package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

const (
	createDisciplineSQL = `
		INSERT INTO disciplines (name)
		VALUES (:name)
		RETURNING id
	`

	getDisciplineByIdSQL = `
		SELECT id, name
		FROM disciplines
		WHERE id = $1
	`

	updateDisciplineByIdSQL = `
		UPDATE disciplines
		SET name = $1
		WHERE id = $2
		RETURNING id, name
	`

	deleteDisciplineByIdSQL = `
		DELETE FROM disciplines
		WHERE id = $1
	`
)

func (r *PGRepo) CreateDiscipline(
	ctx context.Context,
	model *disciplines.CreateDiscipline,
) (*disciplines.Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", createDisciplineSQL))

	newDiscipline := disciplines.Discipline{
		Name: model.Name,
	}

	rows, err := r.db.NamedQueryContext(ctx, createDisciplineSQL, newDiscipline)
	if err != nil {
		return nil, fmt.Errorf("create discipline: insert in db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newDiscipline.ID); err != nil {
			return nil, fmt.Errorf(
				"create discipline: scan discipline id: %w",
				err,
			)
		}
	}

	return &newDiscipline, nil
}

func (r *PGRepo) GetDisciplineByID(
	ctx context.Context,
	id uuid.UUID,
) (*disciplines.Discipline, error) {
	zap.L().Debug("Executing query", zap.String("query", getDisciplineByIdSQL))

	var discipline disciplines.Discipline
	err := r.db.GetContext(ctx, &discipline, getDisciplineByIdSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &discipline, nil
}

func (r *PGRepo) UpdateDisciplineByID(
	ctx context.Context,
	id uuid.UUID,
	model *disciplines.PatchDiscipline,
) (*disciplines.Discipline, error) {
	zap.L().
		Debug("Executing query", zap.String("query", updateDisciplineByIdSQL))

	row := r.db.QueryRowxContext(
		ctx,
		updateDisciplineByIdSQL,
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

func (r *PGRepo) DeleteDisciplineByID(ctx context.Context, id uuid.UUID) error {
	zap.L().
		Debug("Executing query", zap.String("query", deleteDisciplineByIdSQL))

	res, err := r.db.ExecContext(ctx, deleteDisciplineByIdSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
