package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

const (
	createCourseSQL = `
		INSERT INTO courses (discipline_id, owner_id, name, display_name, created_at)
		VALUES (:discipline_id, :owner_id, :name, :display_name, :created_at)
		RETURNING id
	`

	getCourseByIDSQL = `
		SELECT id, discipline_id, owner_id, name, display_name, created_at, deleted_at
		FROM courses
		WHERE id = $1 AND deleted_at IS NULL
	`
	getCourseByNameSQL = `
		SELECT id, discipline_id, owner_id, name, display_name, created_at, deleted_at
		FROM courses
		WHERE name = $1 AND deleted_at IS NULL
	`

	updateCourseByIDSQL = `
		UPDATE courses
		SET owner_id = COALESCE($1, owner_id), name = COALESCE($2, name), display_name = COALESCE($3, display_name)
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id, discipline_id, owner_id, name, display_name, created_at, deleted_at
	`

	deleteCourseByIDSQL = `
		UPDATE courses
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
)

func (r *PGRepo) ExecInTx(
	ctx context.Context,
	fn func(tx *sqlx.Tx) error,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("tx begin failed: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			rollback(tx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		rollback(tx)
		return err
	}
	return tx.Commit()
}

// CreateCourse creates a course with its owner as its first teacher.
func (r *PGRepo) CreateCourse(
	ctx context.Context,
	model *courses.CreateCourse,
	ownerID uuid.UUID,
) (*courses.Course, error) {
	var createdCourse *courses.Course

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var txErr error
		createdCourse, txErr = r.createCourseTx(ctx, tx, model, ownerID)
		if txErr != nil {
			return fmt.Errorf("create course: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return createdCourse, nil
}

func (r *PGRepo) createCourseTx(
	ctx context.Context,
	tx *sqlx.Tx,
	model *courses.CreateCourse,
	ownerID uuid.UUID,
) (*courses.Course, error) {
	newCourse := courses.Course{
		DisciplineID: model.DisciplineID,
		OwnerID:      ownerID,
		Name:         model.Name,
		DisplayName:  model.DisplayName,
		CreatedAt:    time.Now(),
	}

	stmt, err := tx.PrepareNamedContext(ctx, createCourseSQL)
	if err != nil {
		return nil, fmt.Errorf("tx prepare named statement for course: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			zap.L().
				Error("failed to close statement", zap.Error(err))
		}
	}()

	var newID uuid.UUID
	if err := stmt.GetContext(ctx, &newID, newCourse); err != nil {
		return nil, fmt.Errorf("tx get course id: %w", err)
	}
	newCourse.ID = newID

	courseMember := membership.Member{
		UserID:   ownerID,
		CourseID: newCourse.ID,
		Role:     membership.TeacherRole,
		IsActive: true,
	}

	if _, err := tx.NamedExecContext(
		ctx,
		createCourseMemberSQL,
		courseMember,
	); err != nil {
		return nil, fmt.Errorf("insert course member: %w", err)
	}

	teacher := membership.Teacher{
		UserID:     ownerID,
		CourseID:   newCourse.ID,
		PromotedBy: ownerID,
		PromotedAt: time.Now(),
		IsActive:   true,
	}

	if _, err := tx.NamedExecContext(
		ctx,
		createTeacherSQL,
		teacher,
	); err != nil {
		return nil, fmt.Errorf("insert teacher: %w", err)
	}

	return &newCourse, nil
}

func (r *PGRepo) GetCourseByID(
	ctx context.Context,
	id uuid.UUID,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByIDSQL))

	var course courses.Course
	err := r.db.GetContext(ctx, &course, getCourseByIDSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

func (r *PGRepo) GetCourseByName(ctx context.Context, name string) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByNameSQL))

	var course courses.Course
	if err := r.db.GetContext(ctx, &course, getCourseByNameSQL, name); err != nil {
		return nil, err
	}
	return &course, nil
}

// GetPaginatedCourses lists courses in a stable, gapless order using keyset
// (seek) pagination. The cursor condition must compare the same columns the
// results are ordered by - here (name, id), with id as a tie-breaker since
// name alone is not unique - or pages can silently skip or repeat rows: a
// cursor on a column that isn't the sort key partitions the *unsorted* table,
// which has no consistent relationship to a page boundary drawn by name.
func (r *PGRepo) GetPaginatedCourses(
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
	lastName string,
	filter courses.CourseFilter,
) ([]courses.Course, error) {
	join := ""
	conditions := []string{"(c.name, c.id) > ($2, $3)", "c.deleted_at IS NULL"}
	args := []any{limit, lastName, lastID}

	if filter.DisciplineID != uuid.Nil {
		args = append(args, filter.DisciplineID)
		conditions = append(conditions, fmt.Sprintf("c.discipline_id = $%d", len(args)))
	}

	if filter.IsTeacher || filter.IsStudent {
		var roles []string
		if filter.IsTeacher {
			roles = append(roles, "'TEACHER'")
		}
		if filter.IsStudent {
			roles = append(roles, "'STUDENT'")
		}

		join = "JOIN course_members cm ON cm.course_id = c.id"
		args = append(args, filter.UserID)
		conditions = append(conditions,
			fmt.Sprintf("cm.user_id = $%d", len(args)),
			fmt.Sprintf("cm.role IN (%s)", strings.Join(roles, ", ")),
		)
	}

	getCoursesByFilter := fmt.Sprintf(`
		SELECT c.id, c.discipline_id, c.owner_id, c.name, c.display_name, c.created_at
		FROM courses c
		%s
		WHERE %s
		ORDER BY c.name, c.id
		LIMIT $1
	`, join, strings.Join(conditions, " AND "))

	zap.L().Debug("Executing query", zap.String("query", getCoursesByFilter))
	var course courses.Course
	var courseList []courses.Course
	rows, err := r.db.QueryxContext(ctx, getCoursesByFilter, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
	ctx context.Context,
	id uuid.UUID,
	update *courses.UpdateCourse,
) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", updateCourseByIDSQL))

	var course courses.Course
	err := r.db.QueryRowxContext(ctx, updateCourseByIDSQL, update.OwnerID, update.Name, update.DisplayName, id).
		StructScan(&course)
	if err != nil {
		return nil, err
	}

	return &course, nil
}

// DeleteCourseByID soft-deletes a course. Its blocks stay as they are:
// every block query starts from the live course row and finds none.
func (r *PGRepo) DeleteCourseByID(ctx context.Context, id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseByIDSQL))

	res, err := r.db.ExecContext(ctx, deleteCourseByIDSQL, id)
	if err != nil {
		return fmt.Errorf("soft delete course: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("soft delete course rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
