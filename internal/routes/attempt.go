package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/attempt"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AttemptHandler interface {
	CreateAttempt(c fuego.ContextWithBody[attempt.MakeAttempt]) (*attempt.Attempt, error)
	GetAttempt(c fuego.ContextNoBody) (*attempt.AttemptResponse, error)
	GradeAttempt(c fuego.ContextWithBody[attempt.Attempt]) (*attempt.RewiewAttempt, error)
	UpdatedAttempt(c fuego.ContextWithBody[attempt.AttemptUpdate]) (*attempt.Attempt, error)
	DeleteAttempt(c fuego.ContextNoBody) (any, error)
}

type attemptHandler struct {
	repo   attempt.AttemptRepository
	logger *zap.Logger
}

func NewAttemptHandler(repo attempt.AttemptRepository, logger *zap.Logger) AttemptHandler {
	return &attemptHandler{
		repo: repo,
	}
}

func (h *attemptHandler) CreateAttempt(c fuego.ContextWithBody[attempt.MakeAttempt]) (*attempt.Attempt, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}
	attempt, err := h.repo.CreateAttempt(c.Context(), &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return attempt, nil
}

func (h *attemptHandler) GetAttempt(c fuego.ContextNoBody) (*attempt.AttemptResponse, error) {
	id, err := uuid.Parse(c.PathParam("id"))
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	attempt, err := h.repo.GetAttempt(c.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return attempt, nil

}

func (h *attemptHandler) GradeAttempt(c fuego.ContextWithBody[attempt.Attempt]) (*attempt.RewiewAttempt, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	rewiewAttempt, err := h.repo.GradeAttempt(c.Context(), &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return rewiewAttempt, nil

}

func (h *attemptHandler) UpdatedAttempt(c fuego.ContextWithBody[attempt.AttemptUpdate]) (*attempt.Attempt, error) {
	body, err := c.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	attempt, err := h.repo.UpdatedAttempt(c.Context(), body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return attempt, nil
}

func (h *attemptHandler) DeleteAttempt(c fuego.ContextNoBody) (any, error) {
	id, err := uuid.Parse(c.PathParam("id"))
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	err = h.repo.DeleteAttempt(c.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
