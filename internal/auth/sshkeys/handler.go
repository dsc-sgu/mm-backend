package sshkeys

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type AddInput struct{ Body AddSSHKey }

func (h *Handler) Add(ctx context.Context, input *AddInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.service.Add(userID, input.Body); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

type DeleteInput struct {
	Fingerprint string `path:"fingerprint"`
}

func (h *Handler) Delete(ctx context.Context, input *DeleteInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	if err := h.service.Delete(userID, input.Fingerprint); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}
