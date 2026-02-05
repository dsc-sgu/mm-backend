package courses

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrCourseMemberNotFound   = errors.New("user is not a course member")
)

func (s *Service) CreateInvite(
	ctx context.Context,
	model *CreateInvite,
	createdBy uuid.UUID,
) (*Invite, error) {
	role, err := s.GetUserRole(ctx, createdBy, model.CourseID)
	if err != nil {
		return nil, fmt.Errorf("create invite: checking permissions: %w", err)
	}
	if role == nil {
		return nil, ErrCourseMemberNotFound
	}
	if *role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return s.Repo.CreateInvite(model, createdBy)
}
