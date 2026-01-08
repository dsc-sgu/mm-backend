package users

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/password"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/google/uuid"
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
	body LoginModel,
) (LoginResponse, http.Cookie, error) {
	user, err := c.GetUserByEmail(ctx, body.Email)
	if err != nil {
		return LoginResponse{}, http.Cookie{}, fmt.Errorf("getting user: %w", err)
	}

	if user == nil {
		return LoginResponse{}, http.Cookie{}, ErrUserNotFound
	}

	if !password.Valid(body.Password, user.PasswordHash, user.PasswordSalt) {
		return LoginResponse{}, http.Cookie{}, ErrWrongCredentials
	}

	s, err := c.sessionRepo.Create(user.Id, c.cookieConfig.SessionLifetime)
	if err != nil {
		return LoginResponse{}, http.Cookie{}, fmt.Errorf("creating session: %w", err)
	}

	response := LoginResponse{
		SessionId: s.Id,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
		UserId:    user.Id,
	}

	cookie := http.Cookie{
		Name:     session.CookieName,
		Value:    s.Id.String(),
		Path:     c.cookieConfig.Path,
		MaxAge:   c.cookieConfig.SessionLifetime,
		Domain:   c.cookieConfig.Domain,
		Secure:   c.cookieConfig.Secure,
		HttpOnly: c.cookieConfig.HttpOnly,
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

	if err := c.sessionRepo.DeleteById(userID); err != nil {
		return ErrSessionNotFound
	} else {
		return nil
	}
}
