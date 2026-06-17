package routes

import (
	"fmt"
	"io"
	"strconv"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

type AttemptController struct {
	attemptService *attempt.Service
	taskService    *tasks.Service
}

func NewAttemptController(
	svc *attempt.Service,
	taskSvc *tasks.Service,
) *AttemptController {
	return &AttemptController{
		svc,
		taskSvc,
	}
}

func (c *AttemptController) GetDiff(
	ctx fuego.ContextNoBody,
) ([]string, error) {
	AttemptID1 := ctx.QueryParam("id1")
	AttemptID2 := ctx.QueryParam("id2")

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

func (c *AttemptController) PushAttempt(ctx fuego.ContextNoBody) (string, error) {
	courseID, err := uuid.Parse(ctx.QueryParam("courseID"))
	if err != nil {
		return "", fuego.BadRequestError{
			Detail: fmt.Errorf("parsing courseID: %w", err).Error(),
		}
	}

	taskGroupID, err := uuid.Parse(ctx.QueryParam("taskGroupID"))
	if err != nil {
		return "", fuego.BadRequestError{
			Detail: fmt.Errorf("parsing taskGroupID: %w", err).Error(),
		}
	}

	taskPositionStr := ctx.QueryParam("taskPosition")
	taskPosition := 1
	if taskPositionStr != "" {
		taskPosition, err = strconv.Atoi(taskPositionStr)
		if err != nil || taskPosition < 1 {
			return "", fuego.BadRequestError{
				Detail: fmt.Errorf("invalid taskPosition: %w", err).Error(),
			}
		}
	}

	participantID := session.UserIDFromContext(ctx.Context())

	// Resolve task ID from taskGroupID + taskPosition
	task, err := c.taskService.GetTaskByPosition(ctx.Context(), taskGroupID, taskPosition)
	if err != nil {
		return "", fuego.BadRequestError{
			Detail: fmt.Errorf("task not found at position %d: %w", taskPosition, err).Error(),
		}
	}

	zipData, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return "", fuego.BadRequestError{
			Detail: fmt.Errorf("reading body: %w", err).Error(),
		}
	}

	commitHash, err := c.attemptService.PushAttempt(courseID, taskGroupID, task.ID, participantID, zipData)
	if err != nil {
		return "", fuego.InternalServerError{Detail: err.Error()}
	}

	return commitHash, nil
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
