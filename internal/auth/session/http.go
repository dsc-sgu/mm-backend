package session

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const CookieName = "SESSION_ID"

func CheckHTTPReq(
	cookie *http.Cookie,
	sessionRepo Repo,
	logger *zap.Logger,
) (*Model, error) {
	u, err := uuid.Parse(cookie.Value)
	if err != nil {
		return nil, err
	}

	session, err := sessionRepo.GetByID(u)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, errors.New("session not found")
	}

	if session.IsExpired() {
		return nil, errors.New("session is expired")
	}

	return session, nil
}
