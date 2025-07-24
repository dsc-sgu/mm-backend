package routes

import (
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/MergeMinds/mm-backend-go/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AttemptHandler interface {
	CreateAttempt(c *gin.Context)
	GetAttempt(c *gin.Context)
	PatchAttempt(c *gin.Context)
	DeleteAttempt(c *gin.Context)
}

type attemptHandler struct {
	service services.AttemptService
}

func NewAttemptHandler(service services.AttemptService) AttemptHandler {
	return &attemptHandler{
		service: service,
	}
}

func (h *attemptHandler) CreateAttempt(c *gin.Context) {
	var attempt dto.CreateAttempt
	if err := c.ShouldBindBodyWithJSON(&attempt); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	created, err := h.service.CreateAttempt(c.Request.Context(), attempt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *attemptHandler) GetAttempt(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	attempt, err := h.service.GetAttempt(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		return
	}
	c.JSON(http.StatusOK, attempt)
}

func (h *attemptHandler) PatchAttempt(c *gin.Context) {
	var attempt dto.CreateAttempt
	if err := c.ShouldBindBodyWithJSON(&attempt); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	updated, err := h.service.PatchAttempt(c.Request.Context(), &attempt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *attemptHandler) DeleteAttempt(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	err = h.service.DeleteAttempt(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
