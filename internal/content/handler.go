package content

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

type CreateBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     CreateBlockCommand
}

type CreateBlockOutput struct {
	Body struct {
		ID uuid.UUID `json:"id"`
	}
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateBlock(ctx context.Context, input *CreateBlockInput) (*CreateBlockOutput, error) {
	actor := EditContext{
		UserID:    session.UserIDFromContext(ctx),
		SessionID: session.SessionIDFromContext(ctx),
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	input.Body.CourseID = input.CourseID
	input.Body.Actor = actor

	created, err := h.service.CreateBlock(ctx, input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	output := &CreateBlockOutput{}
	output.Body.ID = created.BlockID
	return output, nil
}
