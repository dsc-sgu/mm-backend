package blocks

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CreateBlockResponse is the handler-level response for block creation.
type CreateBlockResponse struct {
	ID uuid.UUID `json:"id"`
}

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrSnapshotNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrSnapshotNotDraft):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type GetBlockInput struct {
	BlockID uuid.UUID `path:"block_id"`
}

type GetBlockOutput struct {
	Body *Block
}

func (h *Handler) GetBlock(
	ctx context.Context,
	input *GetBlockInput,
) (*GetBlockOutput, error) {
	block, err := h.svc.GetBlockByID(ctx, input.BlockID)
	if err != nil {
		return nil, handleServiceError(err)
	}
	if block == nil {
		return nil, huma.Error404NotFound("")
	}
	return &GetBlockOutput{Body: block}, nil
}

type CreateBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	Body       CreateBlock
}

type CreateBlockOutput struct {
	Body *CreateBlockResponse
}

func (h *Handler) CreateBlock(
	ctx context.Context,
	input *CreateBlockInput,
) (*CreateBlockOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	input.Body.SnapshotID = input.SnapshotID

	block, err := h.svc.CreateBlock(ctx, &input.Body, userID, sessionID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &CreateBlockOutput{Body: &CreateBlockResponse{ID: block.ID}}, nil
}

type MoveBlockInput struct {
	SnapshotID   uuid.UUID  `path:"snapshot_id"`
	BlockID      uuid.UUID  `path:"block_id"`
	AfterBlockID *uuid.UUID `                   json:"afterBlockID"`
}

func (h *Handler) MoveBlock(
	ctx context.Context,
	input *MoveBlockInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	err := h.svc.MoveBlock(
		ctx,
		input.BlockID,
		input.SnapshotID,
		userID,
		sessionID,
		input.AfterBlockID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type PatchBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
	Body       UpdateBlock
}

type PatchBlockOutput struct {
	Body *Block
}

func (h *Handler) PatchBlock(
	ctx context.Context,
	input *PatchBlockInput,
) (*PatchBlockOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	block, err := h.svc.UpdateBlockContent(
		ctx,
		input.BlockID,
		input.SnapshotID,
		userID,
		sessionID,
		&input.Body,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &PatchBlockOutput{Body: block}, nil
}

type DeleteBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
}

func (h *Handler) DeleteBlock(
	ctx context.Context,
	input *DeleteBlockInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	err := h.svc.DeleteBlockByID(
		ctx,
		input.BlockID,
		input.SnapshotID,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}
