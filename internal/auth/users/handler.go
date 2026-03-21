package users

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc}
}

type LoginInput struct {
	Body LoginUser
}

type LoginOutput struct {
	SetCookie string `header:"Set-Cookie"`
	Body      *LoginResponse
}

func (h *Handler) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
	response, cookie, err := h.svc.Login(ctx, input.Body)
	if err == ErrWrongCredentials {
		return nil, huma.Error400BadRequest("")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	return &LoginOutput{
		SetCookie: cookie.String(),
		Body:      &response,
	}, nil
}

type UserRegisterRequest struct {
	FirstName  string `json:"firstName"  binding:"required"`
	LastName   string `json:"lastName"   binding:"required"`
	Patronymic string `json:"patronymic" binding:"required"`
	Username   string `json:"username"   binding:"required"`
	Email      string `json:"email"      binding:"required"`
	Password   string `json:"password"   binding:"required"`
}

type RegisterInput struct {
	Body UserRegisterRequest
}

type RegisterResponse struct {
	ID uuid.UUID `json:"id"`
}

type RegisterOutput struct {
	Body *RegisterResponse
}

func (h *Handler) Register(ctx context.Context, input *RegisterInput) (*RegisterOutput, error) {
	newUser := NewUser{
		FirstName:  input.Body.FirstName,
		LastName:   input.Body.LastName,
		Patronymic: input.Body.Patronymic,
		Username:   input.Body.Username,
		Email:      input.Body.Email,
		Password:   input.Body.Password,
		Role:       RegularUserRole,
	}

	user, err := h.svc.CreateUser(ctx, &newUser)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	return &RegisterOutput{Body: &RegisterResponse{ID: user.ID}}, nil
}

type LogoutOutput struct {
	SetCookie string `header:"Set-Cookie"`
}

func (h *Handler) Logout(ctx context.Context, _ *struct{}) (*LogoutOutput, error) {
	if err := h.svc.Logout(ctx); err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	clearCookie := h.svc.ClearCookie()
	return &LogoutOutput{SetCookie: clearCookie.String()}, nil
}

type CurrentUser struct {
	FirstName        string    `json:"firstName"`
	LastName         string    `json:"lastName"`
	Patronymic       string    `json:"patronymic"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	Role             UserRole  `json:"role"`
	AvatarURL        string    `json:"avatarURL"`
	SessionExpiresAt time.Time `json:"sessionExpiresAt"`
}

type GetSessionOutput struct {
	Body *CurrentUser
}

func (h *Handler) GetSession(ctx context.Context, _ *struct{}) (*GetSessionOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	user, err := h.svc.GetUserByID(ctx, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if user == nil {
		return nil, huma.Error404NotFound("")
	}

	sessionID := session.SessionIDFromContext(ctx)
	s, err := h.svc.GetSessionByID(sessionID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if s == nil {
		return nil, huma.Error401Unauthorized("")
	}

	return &GetSessionOutput{Body: &CurrentUser{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Patronymic:       user.Patronymic,
		Username:         user.Username,
		Email:            user.Email,
		Role:             user.Role,
		AvatarURL:        user.AvatarURL,
		SessionExpiresAt: s.ExpiresAt,
	}}, nil
}
