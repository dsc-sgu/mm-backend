package users

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/password"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

// NewUser is the input for creating a user, used by both the service and repository layers.
type NewUser struct {
	FirstName  string   `json:"firstName"  binding:"required"`
	LastName   string   `json:"lastName"   binding:"required"`
	Patronymic string   `json:"patronymic" binding:"required"`
	Username   string   `json:"username"   binding:"required"`
	Email      string   `json:"email"      binding:"required"`
	Role       UserRole `json:"role"       binding:"required"`
	Password   string   `json:"password"   binding:"password"`
}

// LoginUser is the input for user login.
type LoginUser struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the response returned after a successful login.
type LoginResponse struct {
	SessionID uuid.UUID `json:"sessionID"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserID    uuid.UUID `json:"userID"`
}

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

	if user == nil ||
		!password.Valid(body.Password, user.PasswordHash, user.PasswordSalt) {
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

func (c *Service) ClearCookie() http.Cookie {
	return http.Cookie{
		Name:     session.CookieName,
		Value:    "",
		Path:     c.cookieConfig.Path,
		MaxAge:   -1,
		Domain:   c.cookieConfig.Domain,
		Secure:   c.cookieConfig.Secure,
		HttpOnly: c.cookieConfig.HTTPOnly,
	}
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
