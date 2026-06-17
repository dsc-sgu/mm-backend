package courses

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type Service struct {
	repo             Repo
	snapshotsService *snapshots.Service
	locksService     *locks.Service
	txManager        TxManager
}

func NewService(
	repo Repo,
	snapshotsService *snapshots.Service,
	locksService *locks.Service,
	txManager TxManager,
) *Service {
	return &Service{
		repo:             repo,
		snapshotsService: snapshotsService,
		locksService:     locksService,
		txManager:        txManager,
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
	ErrCourseNotFound       = errors.New("course not found")
	ErrSnapshotNotFound     = errors.New("snapshot not found")
	ErrDraftNotFound        = errors.New("user draft not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrCourseMemberNotFound = errors.New("user is not a course member")
	ErrInviteNotFound       = errors.New("invite not found")
	ErrInviteRevoked        = errors.New("invite is revoked")
	ErrInviteExpired        = errors.New("invite is expired")
	ErrAlreadyMember        = errors.New(
		"user is already a member of this course",
	)
	ErrSnapshotConflict = errors.New(
		"course version mismatch or modified by another user",
	)
	ErrInvalidTarget = errors.New(
		"target snapshot is invalid or does not belong to this course",
	)
)

// TxManager is an interface for executing transactions on the service level
type TxManager interface {
	ExecInTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error
}

func (s *Service) CheckCourseMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*CourseMember, error) {
	member, err := s.repo.GetCourseMember(ctx, userID, courseID)
	if err != nil {
		return nil, fmt.Errorf("checking permissions: %w", err)
	}
	if member == nil || !member.IsActive {
		return nil, ErrCourseMemberNotFound
	}
	return member, nil
}

func (s *Service) CreateCourse(
	ctx context.Context,
	model *CreateCourse,
	ownerID uuid.UUID,
) (*Course, error) {
	var createdCourse *Course

	err := s.txManager.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var txErr error

		createdCourse, txErr = s.repo.CreateCourse(ctx, tx, model, ownerID)
		if txErr != nil {
			return fmt.Errorf("create course: %w", txErr)
		}

		firstSnapshot := &snapshots.Snapshot{
			CourseID:  createdCourse.ID,
			Version:   1,
			Status:    snapshots.PublishedStatus,
			CreatedBy: ownerID,
			CreatedAt: time.Now(),
		}

		_, txErr = s.snapshotsService.CreateSnapshot(ctx, tx, firstSnapshot)
		if txErr != nil {
			return fmt.Errorf("create initial snapshot: %w", txErr)
		}

		publishModel := &PublishSnapshot{
			CourseID:        createdCourse.ID,
			NewSnapshotID:   firstSnapshot.ID,
			ExpectedVersion: 0,
		}

		txErr = s.repo.PublishSnapshotToCourse(ctx, tx, publishModel)
		if txErr != nil {
			return fmt.Errorf("link initial snapshot: %w", txErr)
		}

		createdCourse.ActiveSnapshotID = firstSnapshot.ID
		createdCourse.Version = 1

		return nil
	})
	if err != nil {
		return nil, err
	}

	return createdCourse, nil
}

func (s *Service) CreateInvite(
	ctx context.Context,
	model *CreateInvite,
	createdBy uuid.UUID,
) (*Invite, error) {
	courseMember, err := s.CheckCourseMember(ctx, createdBy, model.CourseID)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return s.repo.CreateInvite(ctx, model, createdBy)
}

func (s *Service) GetInviteDetails(
	ctx context.Context,
	inviteID uuid.UUID,
) (*InviteDetails, error) {
	invite, err := s.repo.GetInviteByID(ctx, inviteID)
	if err != nil {
		return nil, fmt.Errorf("get invite details: get invite: %w", err)
	}
	if invite == nil {
		return nil, ErrInviteNotFound
	}

	course, err := s.repo.GetCourseByID(ctx, invite.CourseID)
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
	if time.Now().After(invite.ExpiresAt) {
		return uuid.Nil, ErrInviteExpired
	}

	courseMember, err := s.repo.GetCourseMember(ctx, userID, invite.CourseID)
	if err != nil {
		return uuid.Nil, fmt.Errorf(
			"join by invite: check existing role: %w",
			err,
		)
	}
	if courseMember != nil && courseMember.IsActive {
		return uuid.Nil, ErrAlreadyMember
	}

	if err := s.repo.EnrollUserByInvite(ctx, userID, invite); err != nil {
		return uuid.Nil, fmt.Errorf("join by invite: enroll user: %w", err)
	}

	return invite.CourseID, nil
}

// LockAndInitDraft initializes draft and sets pessimistic lock for the course
func (s *Service) LockAndInitDraft(
	ctx context.Context,
	session *locks.LockSession,
) (*InitLockResult, error) {
	// Check user permissions
	courseMember, err := s.CheckCourseMember(
		ctx,
		session.UserID,
		session.CourseID,
	)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	// Try to set pessimistic lock for the course
	_, err = s.locksService.SetLock(ctx, session)
	if err != nil {
		return nil, err
	}

	draft, err := s.snapshotsService.FindUserDraft(
		ctx,
		session.CourseID,
		session.UserID,
	)
	if err != nil {
		return nil, fmt.Errorf("lock and init: find draft: %w", err)
	}

	course, err := s.repo.GetCourseByID(ctx, session.CourseID)
	if err != nil {
		return nil, fmt.Errorf("lock and init: get course: %w", err)
	}
	if course == nil {
		return nil, ErrCourseNotFound
	}

	// If user's draft already exists, check that it is still actual
	if draft != nil {
		if draft.Version == course.Version+1 {
			// Case 1: Draft exists and is actual
			return &InitLockResult{
				InitType:        InitTypeRestored,
				DraftSnapshotID: draft.ID,
			}, nil
		}
		// Case 2: Draft exists but it version is stale
		return &InitLockResult{
			InitType:        InitTypeStaleConflict,
			DraftSnapshotID: draft.ID,
		}, nil
	}

	// Case 3: Draft does not exist, create a new one
	var newDraft *snapshots.Snapshot
	err = s.txManager.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var txErr error
		targetVersion := course.Version + 1
		newDraft, txErr = s.snapshotsService.CreateDraftFromActual(
			ctx,
			tx,
			session.CourseID,
			targetVersion,
			session.UserID,
			course.ActiveSnapshotID,
		)
		return txErr
	})
	if err != nil {
		return nil, fmt.Errorf("lock and init: create draft tx: %w", err)
	}

	return &InitLockResult{
		InitType:        InitTypeNew,
		DraftSnapshotID: newDraft.ID,
	}, nil
}

// SwitchSnapshot replaces the draft content with the selected snapshot
func (s *Service) SwitchSnapshot(
	ctx context.Context,
	session *locks.LockSession,
	targetSnapshotID uuid.UUID,
) error {
	// Check active lock
	err := s.locksService.ValidateLock(ctx, session)
	if err != nil {
		return err
	}

	// Check that the target snapshot exists and is published
	targetSnapshot, err := s.snapshotsService.GetSnapshotByID(
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
	draft, err := s.snapshotsService.FindUserDraft(
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

	// Replace draft blocks with target
	return s.txManager.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		return s.snapshotsService.SwitchSnapshotContent(
			ctx,
			tx,
			draft.ID,
			targetSnapshotID,
		)
	})
}

// PublishDraft checks optimistic lock, fixes editings,
// sets the draft as active snapshot in the course and unlock the course
func (s *Service) PublishDraft(
	ctx context.Context,
	session *locks.LockSession,
	draftSnapshotID uuid.UUID,
) error {
	// Check active lock
	err := s.locksService.ValidateLock(ctx, session)
	if err != nil {
		return err
	}

	draft, err := s.snapshotsService.GetSnapshotByID(ctx, draftSnapshotID)
	if err != nil {
		return fmt.Errorf("publish draft: get draft: %w", err)
	}
	if draft == nil || draft.Status != snapshots.DraftStatus ||
		draft.CourseID != session.CourseID || draft.CreatedBy != session.UserID {
		return ErrDraftNotFound
	}

	publishSnapshot := &PublishSnapshot{
		CourseID:        session.CourseID,
		NewSnapshotID:   draftSnapshotID,
		ExpectedVersion: draft.Version - 1,
	}

	err = s.txManager.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		txErr := s.repo.PublishSnapshotToCourse(
			ctx,
			tx,
			publishSnapshot,
		)
		if txErr != nil {
			if errors.Is(txErr, sql.ErrNoRows) {
				return ErrSnapshotConflict
			}
			return txErr
		}

		return s.snapshotsService.UpdateSnapshotStatus(
			ctx,
			tx,
			draftSnapshotID,
			snapshots.PublishedStatus,
		)
	})
	if err != nil {
		return err
	}

	if err := s.locksService.Unlock(ctx, session); err != nil {
		zap.L().Error("failed to remove course lock", zap.Error(err))
	}

	return nil
}

// CancelEdit resets the draft and deletes pessimistic lock
func (s *Service) CancelEdit(
	ctx context.Context,
	session *locks.LockSession,
) error {
	draft, err := s.snapshotsService.FindUserDraft(
		ctx,
		session.CourseID,
		session.UserID,
	)
	if err != nil {
		return fmt.Errorf("cancel edit: find draft: %w", err)
	}

	if draft == nil {
		return s.locksService.Unlock(ctx, session)
	}

	err = s.txManager.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		return s.snapshotsService.DiscardDraft(ctx, tx, draft.ID)
	})
	if err != nil {
		return fmt.Errorf("cancel edit: discard draft tx: %w", err)
	}

	return s.locksService.Unlock(ctx, session)
}

func (s *Service) GetCourseMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*CourseMember, error) {
	return s.repo.GetCourseMember(ctx, userID, courseID)
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

func (s *Service) UpdateCourseByID(
	ctx context.Context,
	id, userID uuid.UUID,
	update *UpdateCourse,
) (*Course, error) {
	courseMember, err := s.CheckCourseMember(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != TeacherRole {
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
	if courseMember.Role != TeacherRole {
		return ErrPermissionDenied
	}
	return s.repo.DeleteCourseByID(ctx, id)
}

func (s *Service) GetSnapshotByID(
	ctx context.Context,
	snapshotID, userID uuid.UUID,
) (*snapshots.Snapshot, error) {
	snapshot, err := s.snapshotsService.GetSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	if snapshot == nil {
		return nil, ErrSnapshotNotFound
	}

	courseMember, err := s.CheckCourseMember(ctx, userID, snapshot.CourseID)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}

	return snapshot, nil
}

func (s *Service) GetPublishedSnapshots(
	ctx context.Context,
	courseID, userID uuid.UUID,
) ([]*snapshots.Snapshot, error) {
	courseMember, err := s.CheckCourseMember(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if courseMember.Role != TeacherRole {
		return nil, ErrPermissionDenied
	}
	return s.snapshotsService.GetPublishedSnapshotsByCourseID(ctx, courseID)
}
