package courses

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/checkpoints"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

type Service struct {
	repo              Repo
	checkpoints       checkpoints.Repo
	blockService      *blocks.Service
	membershipService *membership.Service
	userService       *users.Service
}

func NewService(
	repo Repo,
	checkpointRepo checkpoints.Repo,
	blockService *blocks.Service,
	membershipService *membership.Service,
	userService *users.Service,
) *Service {
	return &Service{
		repo:              repo,
		checkpoints:       checkpointRepo,
		blockService:      blockService,
		membershipService: membershipService,
		userService:       userService,
	}
}

var (
	ErrCourseNotFound   = errors.New("course not found")
	ErrPermissionDenied = errors.New("permission denied")
)

// CheckCourseMember is a course-domain-flavored alias for
// membershipService.CheckMember, returning membership.ErrNotFound as-is
// when userID is not an active member of courseID.
func (s *Service) CheckCourseMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*membership.Member, error) {
	return s.membershipService.CheckMember(ctx, userID, courseID)
}

// requireTeacher ensures userID is an active teacher of courseID.
func (s *Service) requireTeacher(
	ctx context.Context,
	userID, courseID uuid.UUID,
) error {
	courseMember, err := s.CheckCourseMember(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if courseMember.Role != membership.TeacherRole {
		return ErrPermissionDenied
	}
	return nil
}

func (s *Service) CreateCourse(
	ctx context.Context,
	model *CreateCourse,
	ownerID uuid.UUID,
) (*Course, error) {
	return s.repo.CreateCourse(ctx, model, ownerID)
}

// Publish publishes every draft block of the teacher in the course, merged
// with what was published meanwhile, and returns the checkpoint recording
// the result.
func (s *Service) Publish(
	ctx context.Context,
	courseID, userID uuid.UUID,
) (*checkpoints.Checkpoint, error) {
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.checkpoints.PublishDrafts(ctx, courseID, userID)
}

// CancelEdit discards every draft block of the teacher in the course.
func (s *Service) CancelEdit(
	ctx context.Context,
	courseID, userID uuid.UUID,
) error {
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return err
	}
	return s.checkpoints.DiscardDrafts(ctx, courseID, userID)
}

func (s *Service) GetPaginatedCourses(
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
	lastName string,
	filter CourseFilter,
) ([]Course, error) {
	return s.repo.GetPaginatedCourses(ctx, limit, lastID, lastName, filter)
}

func (s *Service) GetCourseByID(
	ctx context.Context,
	id uuid.UUID,
) (*Course, error) {
	return s.repo.GetCourseByID(ctx, id)
}

// GetCourseContent returns a course together with its ordered blocks: the
// published ones, or, with draft set, the teacher's own draft versions over
// the published ones.
func (s *Service) GetCourseContent(
	ctx context.Context,
	courseID, userID uuid.UUID,
	draft bool,
) (*CourseContentResponse, error) {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get course content: get course: %w", err)
	}
	if course == nil {
		return nil, ErrCourseNotFound
	}

	linkedBlocks, err := s.blockService.GetCourseBlocks(ctx, courseID, userID, draft)
	if err != nil {
		return nil, err
	}

	return &CourseContentResponse{
		ID:           course.ID,
		DisciplineID: course.DisciplineID,
		Name:         course.Name,
		Owner: resolveUserSummary(
			ctx,
			s.userService,
			course.OwnerID,
		),
		CreatedAt: course.CreatedAt,
		Blocks:    linkedBlocks,
	}, nil
}

func (s *Service) UpdateCourseByID(
	ctx context.Context,
	id, userID uuid.UUID,
	update *UpdateCourse,
) (*Course, error) {
	if err := s.requireTeacher(ctx, userID, id); err != nil {
		return nil, err
	}
	return s.repo.UpdateCourseByID(ctx, id, update)
}

func (s *Service) DeleteCourseByID(
	ctx context.Context,
	id, userID uuid.UUID,
) error {
	if err := s.requireTeacher(ctx, userID, id); err != nil {
		return err
	}
	return s.repo.DeleteCourseByID(ctx, id)
}

// GetCheckpoints lists the publishes of a course, latest first.
func (s *Service) GetCheckpoints(
	ctx context.Context,
	courseID, userID uuid.UUID,
) ([]*checkpoints.Checkpoint, error) {
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.checkpoints.GetCheckpoints(ctx, courseID)
}

// GetCheckpointBlocks returns the course's blocks as a publish left them.
func (s *Service) GetCheckpointBlocks(
	ctx context.Context,
	courseID, checkpointID, userID uuid.UUID,
) ([]*blocks.Block, error) {
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.checkpoints.GetCheckpointBlocks(ctx, courseID, checkpointID)
}

// RestoreCheckpoint publishes the course as it was at the checkpoint.
func (s *Service) RestoreCheckpoint(
	ctx context.Context,
	courseID, checkpointID, userID uuid.UUID,
) (*checkpoints.Checkpoint, error) {
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.checkpoints.RestoreCheckpoint(ctx, courseID, checkpointID, userID)
}
