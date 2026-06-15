package locks

import (
	"context"
	"fmt"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// CheckLockValid checks that the lock is still held by the user.
func (s *Service) CheckLockValid(
	ctx context.Context,
	session *LockSession,
) (bool, error) {
	currentLock, err := s.repo.GetLock(ctx, session.CourseID)
	if err != nil {
		return false, fmt.Errorf("service: check lock valid: %w", err)
	}

	if currentLock == nil {
		return false, nil
	}

	if currentLock.UserID != session.UserID ||
		currentLock.SessionID != session.SessionID {
		return false, nil
	}

	return true, nil
}
