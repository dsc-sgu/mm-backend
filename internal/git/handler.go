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

type AddSSHKeyInput struct {
	Body AddSSHKey
}

func (h *Handler) AddSSHKey(ctx context.Context, input *AddSSHKeyInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.svc.AddSSHKey(userID, &input.Body); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}

type DeleteSSHKeyInput struct {
	Body DeleteSSHKey
}

func (h *Handler) DeleteSSHKey(ctx context.Context, input *DeleteSSHKeyInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.svc.DeleteSSHKey(userID, &input.Body); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}
