package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

const (
	createCourseSQL = `
		INSERT INTO courses (discipline_id, owner_id, name, created_at)
		VALUES (:discipline_id, :owner_id, :name, :created_at)
		RETURNING id
	`

	getCourseByIDSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, version, created_at, deleted_at
		FROM courses
		WHERE id = $1 AND deleted_at IS NULL
	`

	getCourseByNameSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, version, created_at, deleted_at
		FROM courses
		WHERE name = $1 AND deleted_at IS NULL
	`

	getAllCoursesByCourseIDSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, version, created_at, deleted_at
		FROM courses
		WHERE id > $2 AND deleted_at IS NULL
		ORDER BY id
		LIMIT $1
	`

	updateCourseByIDSQL = `
		UPDATE courses
		SET owner_id = COALESCE($1, owner_id), name = COALESCE($2, name)
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, discipline_id, active_snapshot_id, owner_id, name, version, created_at, deleted_at
	`

	publishSnapshotToCourseSQL = `
		UPDATE courses
		SET active_snapshot_id = $1, version = version + 1
 		WHERE id = $2 AND version = $3 AND deleted_at IS NULL
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

// CreateCourseWithInitialSnapshot atomically creates a course together with
// its first published snapshot and links them.
func (r *PGRepo) CreateCourseWithInitialSnapshot(
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

		firstSnapshot := &snapshots.Snapshot{
			CourseID:  createdCourse.ID,
			Version:   1,
			Status:    snapshots.PublishedStatus,
			CreatedBy: ownerID,
			CreatedAt: time.Now(),
		}

		createdSnapshot, txErr := r.CreateSnapshot(ctx, tx, firstSnapshot)
		if txErr != nil {
			return fmt.Errorf("create initial snapshot: %w", txErr)
		}

		txErr = r.publishSnapshotToCourseTx(
			ctx,
			tx,
			createdCourse.ID,
			createdSnapshot.ID,
			0,
		)
		if txErr != nil {
			return fmt.Errorf("link initial snapshot: %w", txErr)
		}

		createdCourse.ActiveSnapshotID = &createdSnapshot.ID
		createdCourse.Version = 1

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
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
	discipline_id uuid.UUID,
) ([]courses.Course, error) {
	whereClause := `WHERE id > $2`
	if discipline_id != uuid.Nil {
		whereClause += fmt.Sprintf(` AND discipline_id='%s'`, discipline_id)
	}

	getAllCoursesByCourseIdSQL := fmt.Sprintf(`
		SELECT id, discipline_id, owner_id, name, created_at
		FROM courses
		%s
		ORDER BY id
		LIMIT $1
	`, whereClause)

	fmt.Println("ABOBA")
	fmt.Println(getAllCoursesByCourseIdSQL)
	fmt.Println("ABOBA")

	zap.L().Debug("Executing query", zap.String("query", getAllCoursesByCourseIdSQL))
	var course courses.Course
	var courseList []courses.Course
	rows, err := r.db.QueryxContext(
		ctx,
		getAllCoursesByCourseIDSQL,
		limit,
		lastID,
	)
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
	err := r.db.QueryRowxContext(ctx, updateCourseByIDSQL, update.OwnerID, update.Name, id).
		StructScan(&course)
	if err != nil {
		return nil, err
	}

	return &course, nil
}

// PublishDraft atomically links the draft snapshot as the course's active
// snapshot (checking the optimistic lock) and marks it published.
func (r *PGRepo) PublishDraft(
	ctx context.Context,
	courseID, draftSnapshotID uuid.UUID,
	expectedVersion int,
	userID, sessionID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.publishSnapshotToCourseTx(
			ctx,
			tx,
			courseID,
			draftSnapshotID,
			expectedVersion,
		); err != nil {
			return err
		}

		if err := r.UpdateSnapshotStatus(
			ctx,
			tx,
			draftSnapshotID,
			snapshots.PublishedStatus,
		); err != nil {
			return err
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", deleteCourseLockSQL))
		if _, err := tx.ExecContext(
			ctx,
			deleteCourseLockSQL,
			courseID,
			userID,
			sessionID,
		); err != nil {
			return fmt.Errorf("tx delete course lock: %w", err)
		}

		return nil
	})
}

func (r *PGRepo) publishSnapshotToCourseTx(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID, newSnapshotID uuid.UUID,
	expectedVersion int,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", publishSnapshotToCourseSQL))

	res, err := tx.ExecContext(
		ctx,
		publishSnapshotToCourseSQL,
		newSnapshotID,
		courseID,
		expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("tx publish course version: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteCourseByID soft-deletes a course together with all of its snapshots
// and blocks, in one transaction.
func (r *PGRepo) DeleteCourseByID(ctx context.Context, id uuid.UUID) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		zap.L().
			Debug("Executing query within transaction", zap.String("query", deleteCourseByIDSQL))

		res, err := tx.ExecContext(ctx, deleteCourseByIDSQL, id)
		if err != nil {
			return fmt.Errorf("tx soft delete course: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("tx soft delete course rows affected: %w", err)
		}
		if affected == 0 {
			return sql.ErrNoRows
		}

		if err := r.DeleteAllSnapshotsByCourseID(ctx, tx, id); err != nil {
			return fmt.Errorf("cascade delete snapshots: %w", err)
		}

		if err := r.DeleteAllBlocksByCourseID(ctx, tx, id); err != nil {
			return fmt.Errorf("cascade delete blocks: %w", err)
		}

		return nil
	})
}
