package locks

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo           Repo
	lockTTLSeconds int
}

func NewService(repo Repo, ttlSeconds int) *Service {
	return &Service{
		repo:           repo,
		lockTTLSeconds: ttlSeconds,
	}
}

var (
	ErrLockHeldByAnother = errors.New(
		"course lock is held by another user or session",
	)
	ErrLockExpired  = errors.New("course lock has expired")
	ErrLockNotFound = errors.New("course lock not found")
)

// ValidateLock checks that the lock is still held by the user.
func (s *Service) ValidateLock(
	ctx context.Context,
	session *LockSession,
) error {
	currentLock, err := s.repo.GetLock(ctx, session.CourseID)
	if err != nil {
		return fmt.Errorf("validate lock: %w", err)
	}

	if currentLock == nil {
		return ErrLockNotFound
	}

	if currentLock.UserID != session.UserID ||
		currentLock.SessionID != session.SessionID {
		return ErrLockHeldByAnother
	}

	if !currentLock.IsValid {
		return ErrLockExpired
	}

	return nil
}

func (s *Service) GetLock(
	ctx context.Context,
	courseID uuid.UUID,
) (*Lock, error) {
	return s.repo.GetLock(ctx, courseID)
}

func (s *Service) SetLock(
	ctx context.Context,
	model *LockSession,
) (*Lock, *uuid.UUID, error) {
	return s.repo.SetLock(ctx, model, s.lockTTLSeconds)
}

func (s *Service) RefreshLock(
	ctx context.Context,
	model *LockSession,
) (*uuid.UUID, error) {
	return s.repo.RefreshLock(ctx, model, s.lockTTLSeconds)
}

func (s *Service) Unlock(ctx context.Context, model *LockSession) error {
	return s.repo.Unlock(ctx, model)
}
