package courses

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

var (
	ErrPermissionDenied     = errors.New("permission denied")
	ErrCourseMemberNotFound = errors.New("user is not a course member")
	ErrInviteNotFound       = errors.New("invite not found")
	ErrInviteRevoked        = errors.New("invite is revoked")
	ErrInviteExpired        = errors.New("invite is expired")
	ErrAlreadyMember        = errors.New(
		"user is already a member of this course",
	)
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

func (s *Service) JoinCourseByInvite(
	ctx context.Context,
	inviteID, userID uuid.UUID,
) error {
	invite, err := s.GetInviteByID(ctx, inviteID)
	if err != nil {
		return fmt.Errorf("join by invite: get invite: %w", err)
	}
	if invite == nil {
		return ErrInviteNotFound
	}
	if invite.IsRevoked {
		return ErrInviteRevoked
	}
	if time.Now().After(invite.ExpiresAt) {
		return ErrInviteExpired
	}

	role, err := s.GetUserRole(ctx, userID, invite.CourseID)
	if err != nil {
		return fmt.Errorf("join by invite: check existing role: %w", err)
	}
	if role != nil {
		return ErrAlreadyMember
	}

	if err := s.EnrollUserByInvite(ctx, userID, invite); err != nil {
		return fmt.Errorf("join by invite: enroll user: %w", err)
	}

	return nil
}
