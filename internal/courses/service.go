package courses

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type Service struct {
	repo              Repo
	snapshotService   *snapshots.Service
	lockService       *locks.Service
	blockService      *blocks.Service
	membershipService *membership.Service
	userService       *users.Service
}

func NewService(
	repo Repo,
	snapshotService *snapshots.Service,
	lockService *locks.Service,
	blockService *blocks.Service,
	membershipService *membership.Service,
	userService *users.Service,
) *Service {
	return &Service{
		repo:              repo,
		snapshotService:   snapshotService,
		lockService:       lockService,
		blockService:      blockService,
		membershipService: membershipService,
		userService:       userService,
	}
}

type InitType string

const (
	InitTypeNew           InitType = "new"
	InitTypeRestored      InitType = "restored"
	InitTypeStaleConflict InitType = "stale_conflict"
)

type InitLockResult struct {
	InitType        InitType  `json:"initType"`
	DraftSnapshotID uuid.UUID `json:"draftSnapshotID"`
}

var (
	ErrCourseNotFound   = errors.New("course not found")
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrDraftNotFound    = errors.New("user draft not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrSnapshotConflict = errors.New(
		"course version mismatch or modified by another user",
	)
	ErrInvalidTarget = errors.New(
		"target snapshot is invalid or does not belong to this course",
	)
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

func (s *Service) CreateCourse(
	ctx context.Context,
	model *CreateCourse,
	ownerID uuid.UUID,
) (*Course, error) {
	return s.repo.CreateCourseWithInitialSnapshot(ctx, model, ownerID)
}

// LockAndInitDraft initializes draft and sets pessimistic lock for the
// course. The returned *uuid.UUID is the current lock holder's user id,
// non-nil only when err is locks.ErrLockHeldByAnother, so the caller can
// look up and surface who is currently editing.
func (s *Service) LockAndInitDraft(
	ctx context.Context,
	session *locks.LockSession,
) (*InitLockResult, *uuid.UUID, error) {
	// Check user permissions
	courseMember, err := s.CheckCourseMember(
		ctx,
		session.UserID,
		session.CourseID,
	)
	if err != nil {
		return nil, nil, err
	}
	if courseMember.Role != membership.TeacherRole {
		return nil, nil, ErrPermissionDenied
	}

	// Try to set pessimistic lock for the course
	_, holderID, err := s.lockService.SetLock(ctx, session)
	if err != nil {
		return nil, holderID, err
	}

	draft, err := s.snapshotService.FindUserDraft(
		ctx,
		session.CourseID,
		session.UserID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("lock and init: find draft: %w", err)
	}

	course, err := s.repo.GetCourseByID(ctx, session.CourseID)
	if err != nil {
		return nil, nil, fmt.Errorf("lock and init: get course: %w", err)
	}
	if course == nil {
		return nil, nil, ErrCourseNotFound
	}

	// If user's draft already exists, check that it is still actual
	if draft != nil {
		if draft.Version == course.Version+1 {
			// Case 1: Draft exists and is actual
			return &InitLockResult{
				InitType:        InitTypeRestored,
				DraftSnapshotID: draft.ID,
			}, nil, nil
		}
		// Case 2: Draft exists but it version is stale
		return &InitLockResult{
			InitType:        InitTypeStaleConflict,
			DraftSnapshotID: draft.ID,
		}, nil, nil
	}

	// Case 3: Draft does not exist, create a new one
	targetVersion := course.Version + 1
	newDraft, err := s.snapshotService.CreateDraftFromActual(
		ctx,
		session.CourseID,
		targetVersion,
		session.UserID,
		*course.ActiveSnapshotID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("lock and init: create draft: %w", err)
	}

	return &InitLockResult{
		InitType:        InitTypeNew,
		DraftSnapshotID: newDraft.ID,
	}, nil, nil
}

// SwitchSnapshot replaces the draft content with the selected snapshot
func (s *Service) SwitchSnapshot(
	ctx context.Context,
	session *locks.LockSession,
	targetSnapshotID uuid.UUID,
) error {
	// Check active lock
	err := s.lockService.ValidateLock(ctx, session)
	if err != nil {
		return err
	}

	// Check that the target snapshot exists and is published
	targetSnapshot, err := s.snapshotService.GetSnapshotByID(
		ctx,
		targetSnapshotID,
	)
	if err != nil {
		return fmt.Errorf("switch snapshot: get target: %w", err)
	}
	if targetSnapshot == nil || targetSnapshot.CourseID != session.CourseID ||
		targetSnapshot.Status != snapshots.PublishedStatus {
		return ErrInvalidTarget
	}

	// Find current user's draft
	draft, err := s.snapshotService.FindUserDraft(
		ctx,
		session.CourseID,
		session.UserID,
	)
	if err != nil {
		return fmt.Errorf("switch snapshot: find current draft: %w", err)
	}
	if draft == nil {
		return ErrDraftNotFound
	}
	if draft.ID == targetSnapshotID {
		return ErrInvalidTarget
	}

	// Replace draft blocks with target
	return s.snapshotService.SwitchSnapshotContent(
		ctx,
		draft.ID,
		targetSnapshotID,
	)
}

// PublishDraft checks optimistic lock, fixes editings,
// sets the draft as active snapshot in the course and unlock the course
func (s *Service) PublishDraft(
	ctx context.Context,
	session *locks.LockSession,
	draftSnapshotID uuid.UUID,
) error {
	// Check active lock
	err := s.lockService.ValidateLock(ctx, session)
	if err != nil {
		return err
	}

	draft, err := s.snapshotService.GetSnapshotByID(ctx, draftSnapshotID)
	if err != nil {
		return fmt.Errorf("publish draft: get draft: %w", err)
	}
	if draft == nil || draft.Status != snapshots.DraftStatus ||
		draft.CourseID != session.CourseID || draft.CreatedBy != session.UserID {
		return ErrDraftNotFound
	}

	err = s.repo.PublishDraft(
		ctx,
		session.CourseID,
		draftSnapshotID,
		draft.Version-1,
		session.UserID,
		session.SessionID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSnapshotConflict
	}
	return err
}

// CancelEdit resets the draft and deletes pessimistic lock
func (s *Service) CancelEdit(
	ctx context.Context,
	session *locks.LockSession,
) error {
	draft, err := s.snapshotService.FindUserDraft(
		ctx,
		session.CourseID,
		session.UserID,
	)
	if err != nil {
		return fmt.Errorf("cancel edit: find draft: %w", err)
	}

	if draft == nil {
		return s.lockService.Unlock(ctx, session)
	}

	if err := s.snapshotService.DiscardDraft(ctx, draft.ID); err != nil {
		return fmt.Errorf("cancel edit: discard draft: %w", err)
	}

	return s.lockService.Unlock(ctx, session)
}

func (s *Service) GetPaginatedCourses(
	ctx context.Context,
	limit int,
	lastID uuid.UUID,
) ([]Course, error) {
	return s.repo.GetPaginatedCourses(ctx, limit, lastID)
}

func (s *Service) GetCourseByID(
	ctx context.Context,
	id uuid.UUID,
) (*Course, error) {
	return s.repo.GetCourseByID(ctx, id)
}

// GetCourseContent returns a course together with its active snapshot's
// ordered blocks (nil if the course has no active snapshot yet).
func (s *Service) GetCourseContent(
	ctx context.Context,
	courseID, userID uuid.UUID,
) (*CourseContentResponse, error) {
	if _, err := s.CheckCourseMember(ctx, userID, courseID); err != nil {
		return nil, err
	}

	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get course content: get course: %w", err)
	}
	if course == nil {
		return nil, ErrCourseNotFound
	}

	var linkedBlocks []*blocks.Block
	if course.ActiveSnapshotID != nil {
		linkedBlocks, err = s.blockService.GetAllBlocksBySnapshotID(
			ctx,
			*course.ActiveSnapshotID,
		)
		if err != nil {
			return nil, fmt.Errorf("get course content: get blocks: %w", err)
		}
	}

	return &CourseContentResponse{
		ID:               course.ID,
		DisciplineID:     course.DisciplineID,
		ActiveSnapshotID: course.ActiveSnapshotID,
		Name:             course.Name,
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
	courseMember, err := s.CheckCourseMember(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != membership.TeacherRole {
		return nil, ErrPermissionDenied
	}
	return s.repo.UpdateCourseByID(ctx, id, update)
}

func (s *Service) DeleteCourseByID(
	ctx context.Context,
	id, userID uuid.UUID,
) error {
	courseMember, err := s.CheckCourseMember(ctx, userID, id)
	if err != nil {
		return err
	}
	if courseMember.Role != membership.TeacherRole {
		return ErrPermissionDenied
	}
	return s.repo.DeleteCourseByID(ctx, id)
}

// getSnapshotForCourse fetches a snapshot, verifying it belongs to courseID
// and that userID may view it. A draft snapshot is additionally only visible
// to whoever currently holds its course's edit lock
func (s *Service) getSnapshotForCourse(
	ctx context.Context,
	snapshotID, courseID, userID, sessionID uuid.UUID,
) (*snapshots.Snapshot, error) {
	snapshot, err := s.snapshotService.GetSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	if snapshot == nil || snapshot.CourseID != courseID {
		return nil, ErrSnapshotNotFound
	}

	courseMember, err := s.CheckCourseMember(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != membership.TeacherRole {
		return nil, ErrPermissionDenied
	}

	if snapshot.Status == snapshots.DraftStatus {
		lockSession := &locks.LockSession{
			CourseID:  snapshot.CourseID,
			UserID:    userID,
			SessionID: sessionID,
		}
		if err := s.lockService.ValidateLock(ctx, lockSession); err != nil {
			return nil, err
		}
	}

	return snapshot, nil
}

// GetCourseSnapshot returns metadata for a single snapshot of a course.
func (s *Service) GetCourseSnapshot(
	ctx context.Context,
	snapshotID, courseID, userID, sessionID uuid.UUID,
) (*snapshots.Snapshot, error) {
	return s.getSnapshotForCourse(ctx, snapshotID, courseID, userID, sessionID)
}

// GetSnapshotBlocks returns all blocks belonging to a snapshot.
func (s *Service) GetSnapshotBlocks(
	ctx context.Context,
	snapshotID, courseID, userID, sessionID uuid.UUID,
) ([]*blocks.Block, error) {
	snapshot, err := s.getSnapshotForCourse(
		ctx,
		snapshotID,
		courseID,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, err
	}

	linkedBlocks, err := s.blockService.GetAllBlocksBySnapshotID(
		ctx,
		snapshot.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("get snapshot blocks: %w", err)
	}

	return linkedBlocks, nil
}

func (s *Service) GetPublishedSnapshots(
	ctx context.Context,
	courseID, userID uuid.UUID,
) ([]*snapshots.Snapshot, error) {
	courseMember, err := s.CheckCourseMember(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != membership.TeacherRole {
		return nil, ErrPermissionDenied
	}
	return s.snapshotService.GetPublishedSnapshotsByCourseID(ctx, courseID)
}
