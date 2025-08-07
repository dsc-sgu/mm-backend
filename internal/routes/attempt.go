package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/attempt"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AttemptHandler interface {
	CreateAttempt(c fuego.ContextWithBody[attempt.MakeAttempt]) (*attempt.Attempt, error)
	GetAttempt(c fuego.ContextNoBody) (attempt.Attempt, error)
	GradeAttempt(c fuego.ContextWithBody[attempt.Attempt], id uuid.UUID) (*attempt.Attempt, error)
	// DeleteAttempt(c fuego.ContextNoBody) (any, error)
}

type attemptHandler struct {
	repo   attempt.AttemptRepository
	logger *zap.Logger
}

func NewAttemptHandler(repo attempt.AttemptRepository, logger *zap.Logger) AttemptHandler {
	return &attemptHandler{
		repo:   repo,
		logger: logger,
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

func (h *attemptHandler) GetAttempt(c fuego.ContextNoBody) (*attempt.Attempt, error) {
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

func (h *attemptHandler) GradeAttempt(c fuego.ContextWithBody[attempt.Attempt], id uuid.UUID) (*attempt.Attempt, error) {
	id, err := uuid.Parse(c.PathParam("id"))
	if err != nil {
		return nil, err
	}

	body, err := c.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	err = h.repo.GradeAttempt(c.Context(), &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil

}

// func (h *attemptHandler) DeleteAttempt(c fuego.ContextNoBody) (any, error) {
// 	pathId := c.PathParam("block_id")

// 	id, err := uuid.Parse(pathId)
// 	if err != nil {
// 		return nil, fuego.InternalServerError{}
// 	}

// 	err = h.repo.DeleteAttempt(id)
// 	if err != nil {
// 		return nil, fuego.InternalServerError{}
// 	}

// 	return nil, nil
// }
