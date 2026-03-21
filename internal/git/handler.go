package git

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc}
}

type AddSshKeyInput struct {
	Body AddSshKey
}

func (h *Handler) AddSshKey(ctx context.Context, input *AddSshKeyInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.svc.AddSshKey(userID, &input.Body); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}

type DeleteSshKeyInput struct {
	Body DeleteSshKey
}

func (h *Handler) DeleteSshKey(ctx context.Context, input *DeleteSshKeyInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.svc.DeleteSshKey(userID, &input.Body); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}
