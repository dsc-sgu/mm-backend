package pg

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const createCourseSql = `
	INSERT INTO course (id, discipline_id, owner_id, name, created_at)
	VALUES (:id, :discipline_id, :owner_id, :name, :created_at)
	RETURNING id
`

func (r *PGRepo) Create(model *CreateCourse, ownerId uuid.UUID) (*Course, error) {
	zap.L().Debug("Executing query", zap.String("query", createCourseSql))

	newCourse := Course{
		Id:           uuid.New(),
		DisciplineId: model.DisciplineId,
		OwnerId:      ownerId,
		Name:         model.Name,
		CreatedAt:    time.Now(),
	}

	rows, err := r.db.NamedQuery(createCourseSql, newCourse)
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

	return &newCourse, nil
}

const getByIdSql = `
    SELECT id, discipline_id, owner_id, name, created_at
    FROM course
    WHERE id = $1
`

func (r *PGRepo) GetById(ctx context.Context, id uuid.UUID) (*Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getByIdSql))

	var course Course
	err := r.db.GetContext(ctx, &course, getByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

const getAllByCourseIdSql = `
    SELECT *
    FROM course
    LIMIT $1 OFFSET $2
`

func (r *PGRepo) GetPaginatedCourses(limit int, offset int) ([]*Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getAllByCourseIdSql))

	var course Course
	var courseList []*Course
	rows, err := r.db.Queryx(getAllByCourseIdSql, limit, offset)
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
		if err := rows.StructScan(&course); err != nil {
			return nil, err
		}
		courseList = append(courseList, &course)
	}
	if err = rows.Err(); err != nil {
		return courseList, err
	}
	return courseList, nil
}

const updateByIdSql = `
    UPDATE course
    SET owner_id = $1, name = $2
    WHERE id = $3
    RETURNING id, discipline_id, owner_id, name, created_at
`

func (r *PGRepo) UpdateById(id uuid.UUID, update *UpdateCourse) (*Course, error) {
	zap.L().Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRowx(
		updateByIdSql,
		update.OwnerId,
		update.Name,
		id,
	)

	var course Course

	err := row.StructScan(&course)
	if err != nil {
		return &course, err
	}

	return &course, nil
}

const deleteByIdSql = `
    DELETE FROM course
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
