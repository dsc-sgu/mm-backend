package routes

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/password"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
)

type UserService struct {
	service users.Service
}

func NewUserService(repo users.Repo) *UserService {
	return &UserService{
		service: *users.NewService(repo),
	}
}

func (svc *UserService) Login(
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextWithBody[users.LoginModel],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	user, err := svc.service.GetUserByEmail(ctx.Context(), body.Email)
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

func (svc *UserService) Register(
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextWithBody[users.RegisterModel],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	createUser := users.CreateModel{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Username:  body.Username,
		Email:     body.Email,
		Password:  body.Password,
		Role:      "USER",
	}

	u, err := svc.service.CreateUser(&createUser)
	if err != nil {
		return nil, fuego.BadRequestError{Detail: err.Error()}
	}

	response := users.RegisterResponse{
		Id: u.Id,
	}

	return &response, nil
}

func (svc *UserService) Logout(
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextNoBody,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	err := sessionRepo.DeleteById(userID)
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

func (svc *UserService) GetSession(
	cookieConfig *cookie.CookieConfig,
	ctx fuego.ContextNoBody,
) (any, error) {
	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	u, err := svc.service.GetUserByID(ctx.Context(), userID)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	if u == nil {
		return nil, fuego.NotFoundError{Title: "Unrelated to user error"}
	}

	return u, nil
}
