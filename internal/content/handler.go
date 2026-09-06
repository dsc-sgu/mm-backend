package content

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
)

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, blocks.ErrSnapshotNotFound),
		errors.Is(err, blocks.ErrBlockNotFound),
		errors.Is(err, ErrTaskGroupNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, blocks.ErrSnapshotNotDraft),
		errors.Is(err, ErrInvalidTaskBlock):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

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
		return nil, handleServiceError(err)
	}
	output := &CreateBlockOutput{}
	output.Body.ID = created.BlockID
	return output, nil
}

type PatchBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
	Body       PatchBlockCommand
}

type PatchBlockOutput struct {
	Body struct {
		Block *blocks.Block `json:"block"`
		Task  *TaskData     `json:"task,omitempty"`
	}
}

func (h *Handler) PatchBlock(ctx context.Context, input *PatchBlockInput) (*PatchBlockOutput, error) {
	actor := EditContext{
		UserID:    session.UserIDFromContext(ctx),
		SessionID: session.SessionIDFromContext(ctx),
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}
	input.Body.CourseID = input.CourseID
	input.Body.SnapshotID = input.SnapshotID
	input.Body.BlockID = input.BlockID
	input.Body.Actor = actor

	patched, err := h.service.PatchBlock(ctx, input.Body)
	if err != nil {
		return nil, handleServiceError(err)
	}
	output := &PatchBlockOutput{}
	output.Body.Block = patched.Block
	output.Body.Task = patched.Task
	return output, nil
}
