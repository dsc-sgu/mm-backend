package disciplines

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// CreateDisciplineResponse is the handler-level response for discipline creation.
type CreateDisciplineResponse struct {
	ID uuid.UUID `json:"id"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc}
}

type CreateDisciplineInput struct {
	Body CreateDiscipline
}

type CreateDisciplineOutput struct {
	Body *CreateDisciplineResponse
}

func (h *Handler) CreateDiscipline(ctx context.Context, input *CreateDisciplineInput) (*CreateDisciplineOutput, error) {
	discipline, err := h.svc.CreateDiscipline(ctx, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &CreateDisciplineOutput{Body: &CreateDisciplineResponse{ID: discipline.ID}}, nil
}

type GetDisciplineInput struct {
	DisciplineID uuid.UUID `path:"discipline_id"`
}

type GetDisciplineOutput struct {
	Body *Discipline
}

func (h *Handler) GetDiscipline(ctx context.Context, input *GetDisciplineInput) (*GetDisciplineOutput, error) {
	discipline, err := h.svc.GetDisciplineByID(ctx, input.DisciplineID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if discipline == nil {
		return nil, huma.Error404NotFound("")
	}
	return &GetDisciplineOutput{Body: discipline}, nil
}

type PatchDisciplineInput struct {
	DisciplineID uuid.UUID `path:"discipline_id"`
	Body         PatchDiscipline
}

type PatchDisciplineOutput struct {
	Body *Discipline
}

func (h *Handler) PatchDiscipline(ctx context.Context, input *PatchDisciplineInput) (*PatchDisciplineOutput, error) {
	discipline, err := h.svc.UpdateDisciplineByID(ctx, input.DisciplineID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &PatchDisciplineOutput{Body: discipline}, nil
}

type DeleteDisciplineInput struct {
	DisciplineID uuid.UUID `path:"discipline_id"`
}

func (h *Handler) DeleteDiscipline(ctx context.Context, input *DeleteDisciplineInput) (*struct{}, error) {
	// TODO: implement course detaching logic
	if err := h.svc.DeleteDisciplineByID(ctx, input.DisciplineID); err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return nil, nil
}
