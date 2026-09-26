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
		errors.Is(err, ErrBlockNotFound),
		errors.Is(err, ErrTaskGroupNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrSnapshotNotDraft),
		errors.Is(err, ErrAfterBlockNotFound),
		errors.Is(err, ErrInvalidBlockForMoveAfter),
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

func (h *Handler) CreateBlock(
	ctx context.Context,
	input *CreateBlockInput,
) (*CreateBlockOutput, error) {
	actor := EditContext{
		UserID:    session.UserIDFromContext(ctx),
		SessionID: session.SessionIDFromContext(ctx),
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	command := CreateBlockCommand{
		CourseID:     input.CourseID,
		BlockType:    input.Body.BlockType,
		Data:         input.Body.Data,
		AfterBlockID: input.Body.AfterBlockID,
		Task:         input.Body.Task,
		Actor:        actor,
	}

	created, err := h.svc.CreateBlock(ctx, command)
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
		Block *Block    `json:"block"`
		Task  *TaskData `json:"task,omitempty"`
	}
}

func (h *Handler) PatchBlock(
	ctx context.Context,
	input *PatchBlockInput,
) (*PatchBlockOutput, error) {
	actor := EditContext{
		UserID:    session.UserIDFromContext(ctx),
		SessionID: session.SessionIDFromContext(ctx),
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	command := PatchBlockCommand{
		CourseID:   input.CourseID,
		SnapshotID: input.SnapshotID,
		BlockID:    input.BlockID,
		BlockType:  input.Body.BlockType,
		Data:       input.Body.Data,
		Task:       input.Body.Task,
		Actor:      actor,
	}

	patched, err := h.svc.PatchBlock(ctx, command)
	if err != nil {
		return nil, handleServiceError(err)
	}
	output := &PatchBlockOutput{}
	output.Body.Block = patched.Block
	output.Body.Task = patched.Task
	return output, nil
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

	block, err := h.svc.GetBlockByID(
		ctx,
		EditContext{UserID: userID, SessionID: sessionID},
		BlockRef{
			BlockID:    input.BlockID,
			CourseID:   input.CourseID,
			SnapshotID: input.SnapshotID,
		},
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetBlockOutput{Body: block}, nil
}

type MoveBlockInput struct {
	CourseID   uuid.UUID `path:"course_id"`
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
	Body       MoveBlock `                   json:"body"`
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
		EditContext{UserID: userID, SessionID: sessionID},
		BlockRef{
			BlockID:    input.BlockID,
			CourseID:   input.CourseID,
			SnapshotID: input.SnapshotID,
		},
		input.Body.AfterBlockID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
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

	err := h.svc.DeleteBlockByID(
		ctx,
		EditContext{UserID: userID, SessionID: sessionID},
		BlockRef{
			BlockID:    input.BlockID,
			CourseID:   input.CourseID,
			SnapshotID: input.SnapshotID,
		},
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}
