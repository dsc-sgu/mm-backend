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
)

const (
	createCourseSQL = `
		INSERT INTO courses (discipline_id, owner_id, name, display_name, created_at)
		VALUES (:discipline_id, :owner_id, :name, :display_name, :created_at)
		RETURNING id
	`

	createCourseMemberSQL = `
		INSERT INTO course_members (user_id, course_id, role, invited_by, is_active)
		VALUES (:user_id, :course_id, :role, :invited_by, :is_active)
	`

	createStudentSQL = `
		INSERT INTO students (user_id, course_id, admission_date, is_active)
		VALUES (:user_id, :course_id, :admission_date, :is_active)
	`

	createTeacherSQL = `
		INSERT INTO teachers (user_id, course_id, promoted_by, promoted_at, is_active)
		VALUES (:user_id, :course_id, :promoted_by, :promoted_at, :is_active)
	`

	createInviteSQL = `
		INSERT INTO invites (course_id, provided_role, created_by, created_at, expires_at, is_revoked)
		VALUES (:course_id, :provided_role, :created_by, :created_at, :expires_at, :is_revoked)
		RETURNING id
	`

	getCourseByIdSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, display_name, version, created_at, deleted_at
		FROM courses
		WHERE id = $1 AND deleted_at IS NULL
	`
	getCourseByNameSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, display_name, version, created_at, deleted_at
		FROM courses
		WHERE name = $1 AND deleted_at IS NULL
	`

	getAllCoursesByCourseIdSQL = `
		SELECT id, discipline_id, active_snapshot_id, owner_id, name, version, created_at, deleted_at
		FROM courses
		WHERE id > $2 AND deleted_at IS NULL
		ORDER BY id
		LIMIT $1
	`

	getCourseMemberSQL = `
		SELECT user_id, course_id, role, invited_by, is_active
		FROM course_members
		WHERE user_id = $1 AND course_id = $2
	`

	getInviteByIdSQL = `
		SELECT id, course_id, provided_role, created_by, created_at, expires_at, is_revoked
		FROM invites
		WHERE id = $1
	`

	updateCourseByIdSQL = `
		UPDATE courses
		SET owner_id = COALESCE($1, owner_id), display_name = COALESCE($2, display_name)
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, discipline_id, active_snapshot_id, owner_id, name, display_name, version, created_at, deleted_at
	`

	publishSnapshotToCourseSQL = `
		UPDATE courses
		SET active_snapshot_id = $1, version = version + 1
 		WHERE id = $2 AND version = $3 AND deleted_at IS NULL
	`

	deleteCourseByIdSQL = `
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
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *PGRepo) CreateCourse(
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

	courseMember := courses.CourseMember{
		UserID:   ownerID,
		CourseID: newCourse.ID,
		Role:     courses.TeacherRole,
		IsActive: true,
	}

	if _, err := tx.NamedExecContext(
		ctx,
		createCourseMemberSQL,
		courseMember,
	); err != nil {
		return nil, fmt.Errorf("insert course member: %w", err)
	}

	teacher := courses.Teacher{
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
	rolledBack = true

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

func (r *PGRepo) GetCourseByName(ctx context.Context, name string) (*courses.Course, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseByNameSQL))

	var course courses.Course
	err := r.db.GetContext(ctx, &course, getCourseByNameSQL, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return &course, err
		}
		return &course, err
	}
	return &course, nil
}

func (r *PGRepo) GetPaginatedCourses(
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
	discipline_id uuid.UUID,
	userID uuid.UUID,
	isTeacher bool,
	isStudent bool,
) ([]courses.Course, error) {
	whereClause := `WHERE id > $2`
	if discipline_id != uuid.Nil {
		whereClause += fmt.Sprintf(` AND discipline_id='%s'`, discipline_id)
	}

	if userID != uuid.Nil {
		if isTeacher {
			whereClause += fmt.Sprintf(` AND id IN (
			SELECT course_id 
			FROM course_members 
			WHERE user_id = '%s' AND role = 'TEACHER')`, userID)
		} else if isStudent {
			whereClause += fmt.Sprintf(` AND id IN (
			SELECT course_id 
			FROM course_members 
			WHERE user_id = '%s' AND role = 'STUDENT')`, userID)
		}
	}

	getCoursesByFilter := fmt.Sprintf(`
		SELECT id, discipline_id, owner_id, name, display_name, created_at
		FROM courses
		%s
		ORDER BY name
		LIMIT $1
	`, whereClause)

	zap.L().Debug("Executing query", zap.String("query", getCoursesByFilter))
	var course courses.Course
	var courseList []courses.Course
	rows, err := r.db.QueryxContext(
		ctx,
		getCoursesByFilter,
		limit,
		lastID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
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
	zap.L().Debug("Executing query", zap.String("query", updateCourseByIdSQL))

	var course courses.Course
	err := r.db.QueryRowxContext(ctx, updateCourseByIdSQL, update.OwnerID, update.DisplayName, id).
		StructScan(&course)
	if err != nil {
		return nil, err
	}

	return &course, nil
}

func (r *PGRepo) PublishSnapshotToCourse(
	ctx context.Context,
	tx *sqlx.Tx,
	model *courses.PublishSnapshot,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", publishSnapshotToCourseSQL))

	res, err := tx.ExecContext(
		ctx,
		publishSnapshotToCourseSQL,
		model.NewSnapshotID,
		model.CourseID,
		model.ExpectedVersion,
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

func (r *PGRepo) DeleteCourseByID(ctx context.Context, id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteCourseByIdSQL))

	res, err := r.db.ExecContext(ctx, deleteCourseByIdSQL, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PGRepo) CreateInvite(
	ctx context.Context,
	model *courses.CreateInvite,
	createdBy uuid.UUID,
) (*courses.Invite, error) {
	zap.L().Debug("Executing query", zap.String("query", createInviteSQL))

	newInvite := courses.Invite{
		CourseID:     model.CourseID,
		ProvidedRole: model.ProvidedRole,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
		ExpiresAt:    model.ExpiresAt,
		IsRevoked:    false,
	}

	rows, err := r.db.NamedQueryContext(ctx, createInviteSQL, newInvite)
	if err != nil {
		return nil, fmt.Errorf("create invite: insert in db: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newInvite.ID); err != nil {
			return nil, fmt.Errorf("create invite: scan invite id: %w", err)
		}
	}

	return &newInvite, nil
}

func (r *PGRepo) GetInviteByID(
	ctx context.Context,
	inviteID uuid.UUID,
) (*courses.Invite, error) {
	zap.L().Debug("Executing query", zap.String("query", getInviteByIdSQL))

	var invite courses.Invite
	err := r.db.GetContext(ctx, &invite, getInviteByIdSQL, inviteID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

func (r *PGRepo) EnrollUserByInvite(
	ctx context.Context,
	userID uuid.UUID,
	invite *courses.Invite,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("enroll user: begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	courseMember := courses.CourseMember{
		UserID:   userID,
		CourseID: invite.CourseID,
		Role:     invite.ProvidedRole,
		InvitedBy: uuid.NullUUID{
			UUID:  invite.ID,
			Valid: true,
		},
		IsActive: true,
	}

	if _, err := tx.NamedExecContext(
		ctx,
		createCourseMemberSQL,
		courseMember,
	); err != nil {
		return fmt.Errorf("enroll user: insert course member in db: %w", err)
	}

	switch invite.ProvidedRole {
	case courses.StudentRole:
		student := courses.Student{
			UserID:        userID,
			CourseID:      invite.CourseID,
			AdmissionDate: time.Now(),
			IsActive:      true,
		}
		if _, err := tx.NamedExecContext(
			ctx,
			createStudentSQL,
			student,
		); err != nil {
			return fmt.Errorf("enroll user: insert student in db: %w", err)
		}
	case courses.TeacherRole:
		teacher := courses.Teacher{
			UserID:     userID,
			CourseID:   invite.CourseID,
			PromotedBy: invite.CreatedBy,
			PromotedAt: time.Now(),
			IsActive:   true,
		}
		if _, err := tx.NamedExecContext(
			ctx,
			createTeacherSQL,
			teacher,
		); err != nil {
			return fmt.Errorf("enroll user: insert teacher in db: %w", err)
		}
	default:
		return fmt.Errorf("enroll user: unknown role %s", invite.ProvidedRole)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("enroll user: commit transaction: %w", err)
	}

	return nil
}

func (r *PGRepo) GetCourseMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*courses.CourseMember, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseMemberSQL))

	if userID == uuid.Nil {
		return nil, fmt.Errorf("user id is nil")
	}
	if courseID == uuid.Nil {
		return nil, fmt.Errorf("course id is nil")
	}

	var courseMember courses.CourseMember
	err := r.db.GetContext(
		ctx,
		&courseMember,
		getCourseMemberSQL,
		userID,
		courseID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &courseMember, nil
}
