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
		SELECT id, discipline_id, owner_id, name, created_at, version
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
		SET name = :name, version = :version
		WHERE id = :id AND version = :version - 1
		RETURNING id, discipline_id, owner_id, name, created_at, version
	`

	deleteCourseByIdSQL = `
		DELETE FROM courses
		WHERE id = $1
	`
)

func (r *PGRepo) CreateCourse(
	ctx context.Context,
	model *courses.CreateCourse,
	ownerID uuid.UUID,
) (*courses.Course, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create course: begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	newCourse := courses.Course{
		DisciplineID: model.DisciplineID,
		OwnerID:      ownerID,
		Name:         model.Name,
		CreatedAt:    time.Now(),
	}

	rows, err := tx.NamedQuery(createCourseSQL, newCourse)
	if err != nil {
		return nil, fmt.Errorf("create course: insert course in db: %w", err)
	}

	if rows.Next() {
		if err := rows.Scan(&newCourse.ID); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				zap.L().Error(closeErr.Error())
			}
			return nil, fmt.Errorf("create course: scan course id: %w", err)
		}
	}
	if err := rows.Close(); err != nil {
		zap.L().Error(err.Error())
	}

	courseMember := courses.CourseMember{
		UserID:   ownerID,
		CourseID: newCourse.ID,
		Role:     courses.TeacherRole,
		IsActive: true,
	}

	if _, err := tx.NamedExecContext(ctx, createCourseMemberSQL, courseMember); err != nil {
		return nil, fmt.Errorf("create course: insert course member: %w", err)
	}

	teacher := courses.Teacher{
		UserID:     ownerID,
		CourseID:   newCourse.ID,
		PromotedBy: ownerID,
		PromotedAt: time.Now(),
		IsActive:   true,
	}

	if _, err := tx.NamedExecContext(ctx, createTeacherSQL, teacher); err != nil {
		return nil, fmt.Errorf("create course: insert teacher: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create course: commit transaction: %w", err)
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
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
) ([]courses.Course, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getAllCoursesByCourseIdSQL))

	var course courses.Course
	var courseList []courses.Course
	rows, err := r.db.QueryxContext(
		ctx,
		getAllCoursesByCourseIdSQL,
		limit,
		lastID,
	)
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
	ctx context.Context,
	id uuid.UUID,
	update *courses.UpdateCourseRequest,
) (*courses.Course, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("update course: begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Get current course and lock the row
	var currentCourse courses.Course
	if err := tx.GetContext(ctx, &currentCourse, "SELECT * FROM courses WHERE id = $1 FOR UPDATE", id); err != nil {
		if err == sql.ErrNoRows {
			return nil, courses.ErrCourseNotFound
		}
		return nil, fmt.Errorf("update course: get course: %w", err)
	}

	// Check version
	if currentCourse.Version != update.Version {
		return nil, courses.ErrVersionMismatch
	}

	// Delete all blocks for the course
	if _, err := tx.ExecContext(ctx, "DELETE FROM blocks WHERE course_id = $1", id); err != nil {
		return nil, fmt.Errorf("update course: delete blocks: %w", err)
	}

	// Create new blocks
	for i, block := range update.Blocks {
		block.CourseID = id
		block.Position = i
		if _, err := tx.NamedExecContext(ctx, createBlockSQL, block); err != nil {
			return nil, fmt.Errorf("update course: create block: %w", err)
		}
	}

	// Update course
	currentCourse.Name = update.Name
	currentCourse.Version++

	rows, err := tx.NamedQuery(updateCourseByIdSQL, currentCourse)
	if err != nil {
		return nil, fmt.Errorf("update course: update course query: %w", err)
	}

	var updatedCourse courses.Course
	if rows.Next() {
		if err := rows.StructScan(&updatedCourse); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				zap.L().Error(closeErr.Error())
			}
			return nil, fmt.Errorf(
				"update course: scan updated course: %w",
				err,
			)
		}
	}
	if err := rows.Close(); err != nil {
		zap.L().Error(err.Error())
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("update course: commit transaction: %w", err)
	}

	return &updatedCourse, nil
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

	if _, err := tx.NamedExecContext(ctx, createCourseMemberSQL, courseMember); err != nil {
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
		if _, err := tx.NamedExecContext(ctx, createStudentSQL, student); err != nil {
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
		if _, err := tx.NamedExecContext(ctx, createTeacherSQL, teacher); err != nil {
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
