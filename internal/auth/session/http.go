package session

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const COOKIE_NAME = "SESSION_ID"

func CheckHTTPReq(cookie *http.Cookie, sessionRepo Repo, logger *zap.Logger) (*Model, error) {

	cookieIdUUID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return nil, err
	}

	session, err := sessionRepo.GetById(cookieIdUUID)

	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, errors.New("Session not found")
	}

	if session.IsExpired() {
		return nil, errors.New("Session is expired")
	}

	return session, nil
}
