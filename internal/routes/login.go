package routes

import (
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
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

// @description Login into account
// @summary Login into account
// @tags auth
// @accept json
// @produce json
// @param request body LoginModel true "Login data for some shit"
// @success 201 {object} LoginSuccessResponse
// @failure 400 {object} apierr.ApiError "Invalid JsOn"
// @failure 401 {object} apierr.ApiError "Wrong credentials"
// @failure 404 {object} apierr.ApiError "User not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /login [POST]
func Login(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig) {
	var loginJson LoginModel
	if err := c.ShouldBindBodyWithJSON(&loginJson); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	user, err := userRepo.GetByEmail(loginJson.Email)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, apierr.NotFound)
		return
	}

	if !password.Valid(loginJson.Password, user.PasswordHash, user.PasswordSalt) {
		c.JSON(http.StatusUnauthorized, apierr.WrongCredentials)
		return
	}

	s, err := sessionRepo.Create(user.Id, cookieConfig.SessionLifetime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
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

	c.JSON(http.StatusCreated, nil)
}

// @description Register a new account
// @summary Register a new account
// @tags auth
// @accept json
// @produce json
// @param request body RegisterModel true "Register payload"
// @success 201 {object} user.OutModel
// @failure 400 {object} apierr.ApiError "Invalid JSON"
// @failure 401 {object} apierr.ApiError "Wrong credentials"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /register [POST]
func Register(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig) {
	var registerJson RegisterModel
	if err := c.ShouldBindBodyWithJSON(&registerJson); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

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
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusCreated, nil)
}

// @description Logout from an account
// @summary Logout from an account
// @tags auth
// @produce json
// @success 201 {object} LoginSuccessResponse
// @failure 401 {object} apierr.ApiError "Cookie not exists"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /logout [POST]
func Logout(c *gin.Context, userRepo user.Repo,
	sessionRepo session.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig) {
	cookie, err := c.Cookie(session.COOKIE_NAME)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.CookieNotExists)
		return
	}

	cookieIdUUID, err := uuid.Parse(cookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.CookieNotExists)
		return
	}

	err = sessionRepo.DeleteById(cookieIdUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.SetCookie(session.COOKIE_NAME, "", -1, "/", "localhost", false, true)

	c.JSON(http.StatusCreated, nil)
}

// @description Get active session
// @summary Get active session
// @tags auth
// @produce json
// @success 200 {object} user.OutModel
// @failure 401 {object} apierr.ApiError "User not found"
// @failure 404 {object} apierr.ApiError "User not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /session [GET]
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
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	if u == nil {
		c.JSON(http.StatusUnauthorized, apierr.UserNotFound)
		return
	}

	c.JSON(http.StatusOK, mapUserToUserOut(u))
}

func mapUserToUserOut(u *user.Model) *user.OutModel {
	return &user.OutModel{
		Id:         u.Id,
		CreatedAt:  u.CreatedAt,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Patronymic: u.Patronymic,
		Username:   u.Username,
		Email:      u.Email,
		Role:       u.Role,
		AvatarUrl:  u.AvatarUrl,
	}
}
