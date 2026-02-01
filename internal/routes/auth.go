package routes

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
)

type UserController struct {
	svc *users.Service
}

func NewUserController(
	svc *users.Service,
) *UserController {
	return &UserController{
		svc,
	}
}

func (c *UserController) Login(
	ctx fuego.ContextWithBody[users.LoginUser],
) (*users.LoginResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	response, cookie, err := c.svc.Login(ctx.Context(), body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	} else {
		ctx.SetCookie(cookie)
		return &response, nil
	}
}

func (c *UserController) Register(
	ctx fuego.ContextWithBody[users.RegisterUser],
) (*users.RegisterResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	createUser := users.CreateUser{
		FirstName:  body.FirstName,
		LastName:   body.LastName,
		Patronymic: body.Patronymic,
		Username:   body.Username,
		Email:      body.Email,
		Password:   body.Password,
		Role:       "USER",
	}

	user, err := c.svc.CreateUser(&createUser)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := users.RegisterResponse{
		ID: user.ID,
	}

	return &response, nil
}

func (c *UserController) Logout(
	ctx fuego.ContextNoBody,
) (any, error) {
	if err := c.svc.Logout(ctx.Context()); err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	} else {
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
}

func (c *UserController) GetSession(
	ctx fuego.ContextNoBody,
) (*users.GetUser, error) {
	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	user, err := c.svc.GetUserByID(ctx.Context(), userID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	if user == nil {
		return nil, fuego.NotFoundError{Title: "Unrelated to user error"}
	}

	response := users.GetUser{
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		Patronymic: user.Patronymic,
		Username:   user.Username,
		Email:      user.Email,
		Role:       user.Role,
		AvatarURL:  user.AvatarURL,
	}

	return &response, nil
}
