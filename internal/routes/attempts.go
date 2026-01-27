package routes

import (
	"fmt"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/attempt"
)

type AttemptController struct {
	attemtpService *attempt.Service
}

func NewAttemptController(
	svc *attempt.Service,
) *AttemptController {
	return &AttemptController{
		svc,
	}
}

func (a *AttemptController) GetDiff(
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

	diff, err := a.attemtpService.GetDiff(id1, id2)

	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("Cannot make diff: %w", err).Error(),
		}
	}

	return diff, err
}
