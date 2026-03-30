package courses

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateCourse(
		ctx context.Context,
		model *CreateCourse,
		ownerID uuid.UUID,
	) (*Course, error)
	GetCourseByID(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(
		ctx context.Context,
		limit int,
		lastID uuid.UUID,
	) ([]Course, error)
	UpdateCourseByID(
		ctx context.Context,
		id uuid.UUID,
		update *UpdateCourseRequest,
	) (*Course, error)
	DeleteCourseByID(ctx context.Context, id uuid.UUID) error
	CreateInvite(
		ctx context.Context,
		model *CreateInvite,
		createdBy uuid.UUID,
	) (*Invite, error)
	GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*Invite, error)
	EnrollUserByInvite(
		ctx context.Context,
		userID uuid.UUID,
		invite *Invite,
	) error
	GetCourseMember(
		ctx context.Context,
		userID, courseID uuid.UUID,
	) (*CourseMember, error)
}
