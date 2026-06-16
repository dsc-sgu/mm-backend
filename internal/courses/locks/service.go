package locks

import (
	"context"
	"fmt"
)

type Service struct {
	Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

// ValidateLock checks that the lock is still held by the user.
func (s *Service) ValidateLock(
	ctx context.Context,
	session *LockSession,
) (bool, error) {
	currentLock, err := s.GetLock(ctx, session.CourseID)
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
