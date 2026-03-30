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
	ErrCourseNotFound       = errors.New("course not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrCourseMemberNotFound = errors.New("user is not a course member")
	ErrInviteNotFound       = errors.New("invite not found")
	ErrInviteRevoked        = errors.New("invite is revoked")
	ErrInviteExpired        = errors.New("invite is expired")
	ErrAlreadyMember        = errors.New(
		"user is already a member of this course",
	)
	ErrVersionMismatch = errors.New("version mismatch")
)

func (s *Service) CreateInvite(
	ctx context.Context,
	model *CreateInvite,
	createdBy uuid.UUID,
) (*Invite, error) {
	courseMember, err := s.GetCourseMember(ctx, createdBy, model.CourseID)
	if err != nil {
		return nil, fmt.Errorf("create invite: checking permissions: %w", err)
	}
	if courseMember == nil {
		return nil, ErrCourseMemberNotFound
	}
	if courseMember.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return s.Repo.CreateInvite(ctx, model, createdBy)
}

func (s *Service) GetInviteDetails(
	ctx context.Context,
	inviteID uuid.UUID,
) (*InviteDetails, error) {
	invite, err := s.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, fmt.Errorf("get invite details: get invite: %w", err)
	}
	if invite == nil {
		return nil, ErrInviteNotFound
	}

	course, err := s.GetCourseByID(ctx, invite.CourseID)
	if err != nil {
		return nil, fmt.Errorf("get invite details: get course: %w", err)
	}
	if course == nil {
		return nil, ErrCourseNotFound
	}

	details := InviteDetails{
		ID:           invite.ID,
		CourseID:     invite.CourseID,
		CourseName:   course.Name,
		ProvidedRole: invite.ProvidedRole,
		CreatedBy:    invite.CreatedBy,
		CreatedAt:    invite.CreatedAt,
		ExpiresAt:    invite.ExpiresAt,
		IsRevoked:    invite.IsRevoked,
	}

	return &details, nil
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

	courseMember, err := s.GetCourseMember(ctx, userID, invite.CourseID)
	if err != nil {
		return fmt.Errorf("join by invite: check existing role: %w", err)
	}
	if courseMember != nil && courseMember.IsActive {
		return ErrAlreadyMember
	}

	if err := s.EnrollUserByInvite(ctx, userID, invite); err != nil {
		return fmt.Errorf("join by invite: enroll user: %w", err)
	}

	return nil
}

func (s *Service) UpdateCourse(
	ctx context.Context,
	courseID uuid.UUID,
	req *UpdateCourseRequest,
) (*Course, error) {
	// TODO: Add permission check
	return s.UpdateCourseByID(ctx, courseID, req)
}
