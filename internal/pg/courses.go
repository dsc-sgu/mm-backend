package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

const (
	createCourseSQL = `
		INSERT INTO courses (discipline_id, owner_id, name, created_at)
	VALUES (:discipline_id, :owner_id, :name, :created_at)
		RETURNING id
	`

	getCourseByIdSQL = `
		SELECT id, discipline_id, owner_id, name, created_at
		FROM courses
		WHERE id = $1
	`

	getCourseByNameSQL = `
    SELECT id, discipline_id, owner_id, name, created_at
    FROM courses
    WHERE name = $1
  `

	getAllCoursesByCourseIdSQL = `
		SELECT id, discipline_id, owner_id, name, created_at
		FROM courses
		WHERE id > $2
		ORDER BY id
		LIMIT $1
	`

	updateCourseByIdSQL = `
		UPDATE courses
		SET owner_id = $1, name = $2
		WHERE id = $3
		RETURNING id, discipline_id, owner_id, name, created_at
	`

	deleteCourseByIdSQL = `
		DELETE FROM courses
		WHERE id = $1
	`
)

func (r *PGRepo) CreateCourse(
	model *courses.CreateCourse,
	ownerID uuid.UUID,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", createCourseSQL))

	newCourse := courses.Course{
		DisciplineID: model.DisciplineID,
		OwnerID:      ownerID,
		Name:         model.Name,
		CreatedAt:    time.Now(),
	}

	rows, err := r.db.NamedQuery(createCourseSQL, newCourse)
	if err != nil {
		return nil, fmt.Errorf("create course: insert in db: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newCourse.ID); err != nil {
			return nil, fmt.Errorf("create course: scan course id: %w", err)
		}
	}

	return &newCourse, nil
}

func (r *PGRepo) GetCourseByID(
	ctx context.Context,
	id uuid.UUID,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByIdSQL))

	var course courses.Course
	err := r.db.GetContext(ctx, &course, getCourseByIdSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

func (r *PGRepo) GetCourseByName(
	ctx context.Context,
	name string,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByNameSQL))

	var course courses.Course
	err := r.db.GetContext(ctx, &course, getCourseByNameSQL, name)
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
	lastID uuid.UUID,
) ([]courses.Course, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getAllCoursesByCourseIdSQL))

	var course courses.Course
	var courseList []courses.Course
	rows, err := r.db.Queryx(getAllCoursesByCourseIdSQL, limit, lastID)
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
		courseList = append(courseList, course)
	}
	if err = rows.Err(); err != nil {
		return courseList, err
	}
	return courseList, nil
}

func (r *PGRepo) UpdateCourseByID(
	id uuid.UUID,
	update *courses.UpdateCourse,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", updateCourseByIdSQL))

	row := r.db.QueryRowx(
		updateCourseByIdSQL,
		update.OwnerID,
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

func (r *PGRepo) DeleteCourseByID(id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseByIdSQL))

	res, err := r.db.Exec(deleteCourseByIdSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
