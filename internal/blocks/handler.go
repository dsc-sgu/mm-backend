package blocks

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
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
	case errors.Is(err, ErrCourseNotFound),
		errors.Is(err, ErrBlockNotFound),
		errors.Is(err, ErrBlockVersionNotFound),
		errors.Is(err, ErrTaskGroupNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, ErrTaskNameTaken),
		errors.Is(err, ErrTaskGroupNameTaken),
		errors.Is(err, ErrDraftVersionConflict):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, membership.ErrPermissionDenied),
		errors.Is(err, membership.ErrNotFound):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrAfterBlockNotFound),
		errors.Is(err, ErrInvalidBlockForMoveAfter),
		errors.Is(err, ErrInvalidBlockData),
		errors.Is(err, ErrInvalidBlockType),
		errors.Is(err, ErrInvalidTaskGroup),
		errors.Is(err, ErrBlockTypeChanged):
		return huma.Error400BadRequest(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type CreateBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     CreateBlockCommand
}

type CreateBlockOutput struct {
	Body struct {
		ID          uuid.UUID  `json:"id"`
		TaskID      *uuid.UUID `json:"taskID,omitempty"`
		TaskGroupID *uuid.UUID `json:"taskGroupID,omitempty"`
	}
}

func (h *Handler) CreateBlock(
	ctx context.Context,
	input *CreateBlockInput,
) (*CreateBlockOutput, error) {
	actor := EditContext{UserID: session.UserIDFromContext(ctx)}
	if actor.UserID == uuid.Nil {
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
	output.Body.TaskID = created.TaskID
	output.Body.TaskGroupID = created.TaskGroupID
	return output, nil
}

type PatchBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	BlockID  uuid.UUID `path:"block_id"`
	Body     PatchBlockCommand
}

type PatchBlockOutput struct {
	Body struct {
		Block *Block `json:"block"`
	}
}

func (h *Handler) PatchBlock(
	ctx context.Context,
	input *PatchBlockInput,
) (*PatchBlockOutput, error) {
	actor := EditContext{UserID: session.UserIDFromContext(ctx)}
	if actor.UserID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	command := PatchBlockCommand{
		CourseID:  input.CourseID,
		BlockID:   input.BlockID,
		Version:   input.Body.Version,
		BlockType: input.Body.BlockType,
		Data:      input.Body.Data,
		Actor:     actor,
	}

	patched, err := h.svc.PatchBlock(ctx, command)
	if err != nil {
		return nil, handleServiceError(err)
	}
	output := &PatchBlockOutput{}
	output.Body.Block = patched.Block
	return output, nil
}

type GetBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	BlockID  uuid.UUID `path:"block_id"`
}

type GetBlockOutput struct {
	Body *Block
}

func (h *Handler) GetBlock(
	ctx context.Context,
	input *GetBlockInput,
) (*GetBlockOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	block, err := h.svc.GetBlockByID(
		ctx,
		userID,
		BlockRef{BlockID: input.BlockID, CourseID: input.CourseID},
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetBlockOutput{Body: block}, nil
}

type MoveBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	BlockID  uuid.UUID `path:"block_id"`
	Body     MoveBlock `                 json:"body"`
}

func (h *Handler) MoveBlock(
	ctx context.Context,
	input *MoveBlockInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	err := h.svc.MoveBlock(
		ctx,
		EditContext{UserID: userID},
		BlockRef{
			BlockID:  input.BlockID,
			CourseID: input.CourseID,
			Version:  input.Body.Version,
		},
		input.Body.AfterBlockID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type DeleteBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	BlockID  uuid.UUID `path:"block_id"`
	// Version is the version the author loaded and deletes from
	Version int `query:"version" required:"true" minimum:"1"`
}

func (h *Handler) DeleteBlock(
	ctx context.Context,
	input *DeleteBlockInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	err := h.svc.DeleteBlockByID(
		ctx,
		EditContext{UserID: userID},
		BlockRef{
			BlockID:  input.BlockID,
			CourseID: input.CourseID,
			Version:  input.Version,
		},
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}
