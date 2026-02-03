package users

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/password"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

type Service struct {
	Repo
	sessionRepo  session.Repo
	cookieConfig *cookie.CookieConfig
}

func NewService(
	repo Repo,
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
) *Service {
	return &Service{repo, sessionRepo, cookieConfig}
}

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrWrongCredentials = errors.New("wrong credentials")
	ErrSessionNotFound  = errors.New("session not found")
)

func (c *Service) Login(
	ctx context.Context,
	body LoginUser,
) (LoginResponse, http.Cookie, error) {
	user, err := c.GetUserByEmail(ctx, body.Email)
	if err != nil {
		return LoginResponse{}, http.Cookie{}, fmt.Errorf(
			"get user: %w",
			err,
		)
	}

	if user == nil {
		return LoginResponse{}, http.Cookie{}, ErrUserNotFound
	}

	if !password.Valid(body.Password, user.PasswordHash, user.PasswordSalt) {
		return LoginResponse{}, http.Cookie{}, ErrWrongCredentials
	}

	s, err := c.sessionRepo.Create(user.ID, c.cookieConfig.SessionLifetime)
	if err != nil {
		return LoginResponse{}, http.Cookie{}, fmt.Errorf(
			"create session: %w",
			err,
		)
	}

	response := LoginResponse{
		SessionID: s.ID,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
		UserID:    user.ID,
	}

	cookie := http.Cookie{
		Name:     session.CookieName,
		Value:    s.ID.String(),
		Path:     c.cookieConfig.Path,
		MaxAge:   c.cookieConfig.SessionLifetime,
		Domain:   c.cookieConfig.Domain,
		Secure:   c.cookieConfig.Secure,
		HttpOnly: c.cookieConfig.HTTPOnly,
	}

	return response, cookie, nil
}

func (c *Service) Logout(
	ctx context.Context,
) error {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return ErrWrongCredentials
	}

	if err := c.sessionRepo.DeleteByID(userID); err != nil {
		return ErrSessionNotFound
	} else {
		return nil
	}
}

func (c *Service) GetSessionByID(
	sessionID uuid.UUID,
) (*session.Session, error) {
	return c.sessionRepo.GetByID(sessionID)
}
