package blocks

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// CreateBlockResponse is the handler-level response for block creation.
type CreateBlockResponse struct {
	ID uuid.UUID `json:"id"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc}
}

type GetBlockInput struct {
	BlockID uuid.UUID `path:"block_id"`
}

type GetBlockOutput struct {
	Body *Block
}

func (h *Handler) GetBlock(ctx context.Context, input *GetBlockInput) (*GetBlockOutput, error) {
	block, err := h.svc.GetBlockByID(ctx, input.BlockID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if block == nil {
		return nil, huma.Error404NotFound("")
	}
	return &GetBlockOutput{Body: block}, nil
}

type CreateBlockInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     CreateBlock
}

type CreateBlockOutput struct {
	Body *CreateBlockResponse
}

func (h *Handler) CreateBlock(ctx context.Context, input *CreateBlockInput) (*CreateBlockOutput, error) {
	input.Body.CourseID = input.CourseID
	block, err := h.svc.CreateBlock(ctx, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &CreateBlockOutput{Body: &CreateBlockResponse{ID: block.ID}}, nil
}

type PatchBlockInput struct {
	BlockID uuid.UUID `path:"block_id"`
	Body    UpdateBlock
}

type PatchBlockOutput struct {
	Body *Block
}

func (h *Handler) PatchBlock(ctx context.Context, input *PatchBlockInput) (*PatchBlockOutput, error) {
	block, err := h.svc.UpdateBlockByID(ctx, input.BlockID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &PatchBlockOutput{Body: block}, nil
}

type UnlinkFromCourseInput struct {
	BlockID  uuid.UUID `path:"block_id"`
	CourseID uuid.UUID `path:"course_id"`
}

type UnlinkFromCourseOutput struct {
	Body *Block
}

func (h *Handler) UnlinkFromCourse(ctx context.Context, input *UnlinkFromCourseInput) (*UnlinkFromCourseOutput, error) {
	block, err := h.svc.UnlinkBlockByID(ctx, input.CourseID, input.BlockID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &UnlinkFromCourseOutput{Body: block}, nil
}

type DeleteBlockInput struct {
	BlockID uuid.UUID `path:"block_id"`
}

func (h *Handler) DeleteBlock(ctx context.Context, input *DeleteBlockInput) (*struct{}, error) {
	if err := h.svc.DeleteBlockByID(ctx, input.BlockID); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}
