package membership

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

var (
	ErrNotFound         = errors.New("user is not a course member")
	ErrPermissionDenied = errors.New("permission denied")
	ErrInviteNotFound   = errors.New("invite not found")
	ErrInviteRevoked    = errors.New("invite is revoked")
	ErrInviteExpired    = errors.New("invite is expired")
	ErrAlreadyMember    = errors.New("user is already a member of this course")
)

func (s *Service) CheckMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*Member, error) {
	member, err := s.repo.GetMember(ctx, userID, courseID)
	if err != nil {
		return nil, fmt.Errorf("checking membership: %w", err)
	}
	if member == nil || !member.IsActive {
		return nil, ErrNotFound
	}
	return member, nil
}

func (s *Service) CreateInvite(
	ctx context.Context,
	model *CreateInvite,
	createdBy uuid.UUID,
) (*Invite, error) {
	member, err := s.CheckMember(ctx, createdBy, model.CourseID)
	if err != nil {
		return nil, err
	}
	if member.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return s.repo.CreateInvite(ctx, model, createdBy)
}

func (s *Service) GetInvitesByCourseID(
	ctx context.Context,
	courseID, userID uuid.UUID,
) ([]Invite, error) {
	member, err := s.CheckMember(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if member.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return s.repo.GetInvitesByCourseID(ctx, courseID)
}

func (s *Service) GetInviteDetails(
	ctx context.Context,
	inviteID uuid.UUID,
) (*InviteDetails, error) {
	details, err := s.repo.GetInviteDetailsByID(ctx, inviteID)
	if err != nil {
		return nil, fmt.Errorf("get invite details: %w", err)
	}
	if details == nil {
		return nil, ErrInviteNotFound
	}

	return details, nil
}

func (s *Service) JoinCourseByInvite(
	ctx context.Context,
	inviteID, userID uuid.UUID,
) (uuid.UUID, error) {
	invite, err := s.repo.GetInviteByID(ctx, inviteID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("join by invite: get invite: %w", err)
	}
	if invite == nil {
		return uuid.Nil, ErrInviteNotFound
	}
	if invite.IsRevoked {
		return uuid.Nil, ErrInviteRevoked
	}
	if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
		return uuid.Nil, ErrInviteExpired
	}

	member, err := s.repo.GetMember(ctx, userID, invite.CourseID)
	if err != nil {
		return uuid.Nil, fmt.Errorf(
			"join by invite: check existing membership: %w",
			err,
		)
	}
	if member != nil && member.IsActive {
		return uuid.Nil, ErrAlreadyMember
	}

	if err := s.repo.EnrollUserByInvite(ctx, userID, invite); err != nil {
		return uuid.Nil, fmt.Errorf("join by invite: enroll user: %w", err)
	}

	return invite.CourseID, nil
}
