package routes

import (
	"fmt"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/attempt"
)

type AttemptController struct {
	attemptService *attempt.Service
}

func NewAttemptController(
	svc *attempt.Service,
) *AttemptController {
	return &AttemptController{
		svc,
	}
}

func (c *AttemptController) GetDiff(
	ctx fuego.ContextNoBody,
) ([]string, error) {
	AttemptID1 := ctx.QueryParam("attemptID1")
	AttemptID2 := ctx.QueryParam("attemptID2")

	id1, err := uuid.Parse(AttemptID1)

	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	id2, err := uuid.Parse(AttemptID2)

	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	diff, err := c.attemptService.GetDiff(id1, id2)

	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("Cannot make diff: %w", err).Error(),
		}
	}

	return diff, nil
}

func (c *AttemptController) GetAttempts(
	ctx fuego.ContextNoBody,
) ([]attempt.Attempt, error) {
	taskId, err := uuid.Parse(ctx.PathParam("task_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	participantId, err := uuid.Parse(ctx.PathParam("participant_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	attemptList, err := c.attemptService.GetAttempts(taskId, participantId)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return attemptList, nil
}
