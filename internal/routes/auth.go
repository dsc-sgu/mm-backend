package routes

import (
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
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	user, err := userRepo.GetByEmail(ctx.Context(), body.Email)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	if user == nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	if !password.Valid(body.Password, user.PasswordHash, user.PasswordSalt) {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	s, err := sessionRepo.Create(user.Id, cookieConfig.SessionLifetime)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	ctx.SetCookie(
		http.Cookie{
			Name:     session.CookieName,
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
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
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
	if err != nil {
		return nil, fuego.BadRequestError{Detail: err.Error()}
	}

	return nil, nil
}

func Logout(
	userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextNoBody,
) (*struct{}, error) {
	cookie, err := ctx.Cookie(session.CookieName)
	if err != nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	u, err := uuid.Parse(cookie.Value)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	err = sessionRepo.DeleteById(u)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	ctx.SetCookie(http.Cookie{
		Name:     session.CookieName,
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
	cookie, err := ctx.Cookie(session.CookieName)
	if err != nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}
	session, err := session.CheckHTTPReq(cookie, sessionRepo, logger)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	u, err := userRepo.GetById(ctx.Context(), session.UserId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	if u == nil {
		return nil, fuego.NotFoundError{Title: "Unrelated to user error"}
	}

	return u, nil
}
