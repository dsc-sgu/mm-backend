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

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrSnapshotNotFound),
		errors.Is(err, ErrBlockNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrSnapshotNotDraft),
		errors.Is(err, ErrAfterBlockNotFound),
		errors.Is(err, ErrInvalidBlockForMoveAfter):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type GetBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
}

type GetBlockOutput struct {
	Body *Block
}

func (h *Handler) GetBlock(
	ctx context.Context,
	input *GetBlockInput,
) (*GetBlockOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	block, err := h.svc.GetBlockByID(ctx, BlockRef{
		BlockID: input.BlockID, CourseID: input.CourseID, SnapshotID: input.SnapshotID,
	}, EditContext{UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetBlockOutput{Body: block}, nil
}

type MoveBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
	Body       MoveBlock `json:"body"`
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

	err := h.svc.MoveBlock(ctx, BlockRef{
		BlockID: input.BlockID, CourseID: input.CourseID, SnapshotID: input.SnapshotID,
	}, EditContext{UserID: userID, SessionID: sessionID}, input.Body.AfterBlockID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type PatchBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
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

	block, err := h.svc.UpdateBlockContent(ctx, BlockRef{
		BlockID: input.BlockID, CourseID: input.CourseID, SnapshotID: input.SnapshotID,
	}, EditContext{UserID: userID, SessionID: sessionID}, &input.Body)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &PatchBlockOutput{Body: block}, nil
}

type DeleteBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
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

	err := h.svc.DeleteBlockByID(ctx, BlockRef{
		BlockID: input.BlockID, CourseID: input.CourseID, SnapshotID: input.SnapshotID,
	}, EditContext{UserID: userID, SessionID: sessionID})
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}
