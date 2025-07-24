package courses

import (
	"context"
	"database/sql"
	"time"

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

const createCourseSql = `
	INSERT INTO course (id, discipline_id, owner_id, name, created_at)
	VALUES (:id, :discipline_id, :owner_id, :name, :created_at)
	RETURNING id
`

func (r *PGRepo) Create(model *CreateCourseType, ownerId uuid.UUID) (*CourseType, error) {

	r.logger.Debug("Executing query", zap.String("query", createCourseSql))

	newCourse := CourseType{
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
	defer rows.Close()

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

func (r *PGRepo) GetById(id uuid.UUID) (*CourseType, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var course CourseType
	err := r.db.GetContext(context.Background(), &course, getByIdSql, id)
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
    LIMIT :limit OFFSET :offset
`

func (r *PGRepo) GetCourselistPage(limit int, offset int) ([]*CourseType, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var course CourseType
	var courseList []*CourseType
	rows, err := r.db.Query(getAllByCourseIdSql, limit, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(
			&course.Id,
			&course.DisciplineId,
			&course.OwnerId,
			&course.Name,
			&course.CreatedAt,
		); err != nil {
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

func (r *PGRepo) UpdateById(id uuid.UUID, update *UpdateCourseType) (*CourseType, error) {
	r.logger.Debug("Executing query", zap.String("query", updateByIdSql))

	row := r.db.QueryRow(
		updateByIdSql,
		update.OwnerId,
		update.Name,
		id,
	)

	var course CourseType

	err := row.Scan(
		&course.Id,
		&course.DisciplineId,
		&course.OwnerId,
		&course.Name,
		&course.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &course, nil
}

const deleteByIdSql = `
    DELETE FROM course
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
