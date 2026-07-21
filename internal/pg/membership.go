package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

const (
	getCourseMemberSQL = `
		SELECT user_id, course_id, role, invited_by, is_active
		FROM course_members
		WHERE user_id = $1 AND course_id = $2
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

	getInviteByIDSQL = `
		SELECT id, course_id, provided_role, created_by, created_at, expires_at, is_revoked
		FROM invites
		WHERE id = $1
	`

	getInviteDetailsByIDSQL = `
		SELECT invites.id, invites.course_id, courses.name AS course_name, invites.provided_role,
		       invites.created_by, invites.created_at, invites.expires_at, invites.is_revoked
		FROM invites
		JOIN courses ON courses.id = invites.course_id
		WHERE invites.id = $1 AND courses.deleted_at IS NULL
	`

	getInvitesByCourseIDSQL = `
		SELECT id, course_id, provided_role, created_by, created_at, expires_at, is_revoked
		FROM invites
		WHERE course_id = $1
		ORDER BY created_at DESC
	`
)

func (r *PGRepo) GetMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*membership.Member, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseMemberSQL))

	if userID == uuid.Nil {
		return nil, fmt.Errorf("user id is nil")
	}
	if courseID == uuid.Nil {
		return nil, fmt.Errorf("course id is nil")
	}

	var courseMember membership.Member
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

func (r *PGRepo) CreateInvite(
	ctx context.Context,
	model *membership.CreateInvite,
	createdBy uuid.UUID,
) (*membership.Invite, error) {
	zap.L().Debug("Executing query", zap.String("query", createInviteSQL))

	newInvite := membership.Invite{
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
) (*membership.Invite, error) {
	zap.L().Debug("Executing query", zap.String("query", getInviteByIDSQL))

	var invite membership.Invite
	err := r.db.GetContext(ctx, &invite, getInviteByIDSQL, inviteID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

func (r *PGRepo) GetInviteDetailsByID(
	ctx context.Context,
	inviteID uuid.UUID,
) (*membership.InviteDetails, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getInviteDetailsByIDSQL))

	var details membership.InviteDetails
	err := r.db.GetContext(ctx, &details, getInviteDetailsByIDSQL, inviteID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &details, nil
}

func (r *PGRepo) GetInvitesByCourseID(
	ctx context.Context,
	courseID uuid.UUID,
) ([]membership.Invite, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getInvitesByCourseIDSQL))

	inviteList := make([]membership.Invite, 0)
	err := r.db.SelectContext(
		ctx,
		&inviteList,
		getInvitesByCourseIDSQL,
		courseID,
	)
	if err != nil {
		return nil, err
	}
	return inviteList, nil
}

func (r *PGRepo) EnrollUserByInvite(
	ctx context.Context,
	userID uuid.UUID,
	invite *membership.Invite,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		courseMember := membership.Member{
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
			return fmt.Errorf(
				"enroll user: insert course member in db: %w",
				err,
			)
		}

		switch invite.ProvidedRole {
		case membership.StudentRole:
			student := membership.Student{
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
		case membership.TeacherRole:
			teacher := membership.Teacher{
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
			return fmt.Errorf(
				"enroll user: unknown role %s",
				invite.ProvidedRole,
			)
		}

		return nil
	})
}
