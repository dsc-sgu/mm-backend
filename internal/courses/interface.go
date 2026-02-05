package courses

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateCourse(model *CreateCourse, ownerID uuid.UUID) (*Course, error)
	GetCourseByID(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(limit int, lastID uuid.UUID) ([]Course, error)
	UpdateCourseByID(id uuid.UUID, update *UpdateCourse) (*Course, error)
	DeleteCourseByID(id uuid.UUID) error
	CreateInvite(
		model *CreateInvite,
		createdBy uuid.UUID,
	) (*Invite, error)
	GetUserRole(
		ctx context.Context,
		userID, courseID uuid.UUID,
	) (*CourseMemberRole, error)
}
