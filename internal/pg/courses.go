package pg

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

const (
	createCourseSql = `
		INSERT INTO course (id, discipline_id, owner_id, name, created_at)
		VALUES (:id, :discipline_id, :owner_id, :name, :created_at)
		RETURNING id
	`

	getCourseByIdSql = `
		SELECT id, discipline_id, owner_id, name, created_at
		FROM course
		WHERE id = $1
	`

	getAllCoursesByCourseIdSql = `
		SELECT id, discipline_id, owner_id, name, created_at
		FROM course
		WHERE id > $2
		ORDER BY id
		LIMIT $1
	`

	updateCourseByIdSql = `
		UPDATE course
		SET owner_id = $1, name = $2
		WHERE id = $3
		RETURNING id, discipline_id, owner_id, name, created_at
	`

	deleteCourseByIdSql = `
		DELETE FROM course
		WHERE id = $1
	`
)

func (r *PGRepo) CreateCourse(
	model *courses.CreateCourse,
	ownerId uuid.UUID,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", createCourseSql))

	newCourse := courses.Course{
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

func (r *PGRepo) GetCourseById(
	ctx context.Context,
	id uuid.UUID,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByIdSql))

	var course courses.Course
	err := r.db.GetContext(ctx, &course, getCourseByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

func (r *PGRepo) GetPaginatedCourses(
	limit int,
	offset int,
) ([]*courses.Course, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getAllCoursesByCourseIdSql))

	var course courses.Course
	var courseList []*courses.Course
	rows, err := r.db.Queryx(getAllCoursesByCourseIdSql, limit, offset)
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

func (r *PGRepo) UpdateCourseById(
	id uuid.UUID,
	update *courses.UpdateCourse,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", updateCourseByIdSql))

	row := r.db.QueryRowx(
		updateCourseByIdSql,
		update.OwnerId,
		update.Name,
		id,
	)

	var course courses.Course

	err := row.StructScan(&course)
	if err != nil {
		return &course, err
	}

	return &course, nil
}

func (r *PGRepo) DeleteCourseById(id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseByIdSql))

	res, err := r.db.Exec(deleteCourseByIdSql, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
