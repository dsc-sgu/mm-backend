package routes

import (
	"errors"

	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/password"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type LoginModel struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterModel struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Email     string `json:"email" binding:"required"`
	Password  string `json:"password" binding:"required"`
}

type LoginSuccessResponse struct {
	Status string `json:"status"`
}

func Login(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig, loginJson *LoginModel) (*struct{}, error) {

	user, err := userRepo.GetByEmail(loginJson.Email)

	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	if !password.Valid(loginJson.Password, user.PasswordHash, user.PasswordSalt) {
		return nil, errors.New("wrong credentials")
	}

	s, err := sessionRepo.Create(user.Id, cookieConfig.SessionLifetime)
	if err != nil {
		return nil, err
	}

	c.SetCookie(
		session.COOKIE_NAME,
		s.Id.String(),
		cookieConfig.SessionLifetime,
		cookieConfig.Path,
		cookieConfig.Domain,
		cookieConfig.Secure,
		cookieConfig.HttpOnly,
	)

	return nil, nil
}

func Register(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig, registerJson *RegisterModel) (*struct{}, error) {

	createUser := user.CreateModel{
		FirstName: registerJson.FirstName,
		LastName:  registerJson.LastName,
		Username:  registerJson.Username,
		Email:     registerJson.Email,
		Password:  registerJson.Password,
		Role:      "USER",
	}

	_, err := userRepo.Create(&createUser)
	if err != nil {
		return nil, err
	}

	return &struct{}{}, nil
}

func Logout(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig) (*struct{}, error) {
	cookie, err := c.Cookie(session.COOKIE_NAME)
	if err != nil {
		return nil, err
	}

	u, err := uuid.Parse(cookie)
	if err != nil {
		return nil, err
	}

	err = sessionRepo.DeleteById(u)
	if err != nil {
		return nil, err
	}

	c.SetCookie(session.COOKIE_NAME, "", -1, "/", "localhost", false, true)

	return &struct{}{}, nil
}

func Session(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig) {
	session, err := session.CheckHTTPReq(c, sessionRepo, logger)
	if err != nil {
		return
	}

	u, err := userRepo.GetById(session.UserId)
	if err != nil {
		return
	}

	if u == nil {
		return
	}

}
