package routes

import (
	"errors"
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/password"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type LoginModel struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterModel struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName"  binding:"required"`
	Username  string `json:"username"  binding:"required"`
	Email     string `json:"email"     binding:"required"`
	Password  string `json:"password"  binding:"required"`
}

type LoginSuccessResponse struct {
	Status string `json:"status"`
}

func Login(userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextWithBody[LoginModel],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	user, err := userRepo.GetByEmail(body.Email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	if !password.Valid(body.Password, user.PasswordHash, user.PasswordSalt) {
		return nil, errors.New("wrong credentials")
	}

	s, err := sessionRepo.Create(user.Id, cookieConfig.SessionLifetime)
	if err != nil {
		return nil, err
	}

	ctx.SetCookie(
		http.Cookie{
			Name:     session.COOKIE_NAME,
			Value:    s.Id.String(),
			Path:     cookieConfig.Path,
			MaxAge:   cookieConfig.SessionLifetime,
			Domain:   cookieConfig.Domain,
			Secure:   cookieConfig.Secure,
			HttpOnly: cookieConfig.HttpOnly,
		},
	)

	return nil, nil
}

func Register(
	userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextWithBody[RegisterModel],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	createUser := user.CreateModel{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Username:  body.Username,
		Email:     body.Email,
		Password:  body.Password,
		Role:      "USER",
	}

	_, err = userRepo.Create(&createUser)

	return nil, err
}

func Logout(
	userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextNoBody,
) (*struct{}, error) {
	cookie, err := ctx.Cookie(session.COOKIE_NAME)
	if err != nil {
		return nil, err
	}

	u, err := uuid.Parse(cookie.Value)
	if err != nil {
		return nil, err
	}

	err = sessionRepo.DeleteById(u)
	if err != nil {
		return nil, err
	}

	ctx.SetCookie(http.Cookie{
		Name:     session.COOKIE_NAME,
		MaxAge:   -1,
		Value:    "",
		Path:     "/",
		Domain:   "localhost",
		Secure:   false,
		HttpOnly: true,
	})

	return nil, nil
}

func Session(
	userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextNoBody,
) (any, error) {
	cookie, err := ctx.Cookie(session.COOKIE_NAME)
	if err != nil {
		return nil, err
	}
	session, err := session.CheckHTTPReq(cookie, sessionRepo, logger)
	if err != nil {
		return nil, err
	}

	u, err := userRepo.GetById(session.UserId)

	if u == nil {
		return nil, errors.New("unexpected error related to user")
	}

	return nil, err
}
