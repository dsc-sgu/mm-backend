package routes

import (
	"fmt"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

type DisciplineController struct {
	svc *disciplines.Service
}

func NewDisciplineController(svc *disciplines.Service) *DisciplineController {
	return &DisciplineController{
		svc,
	}
}

func (c *DisciplineController) CreateDiscipline(
	ctx fuego.ContextWithBody[disciplines.CreateDiscipline],
) (*disciplines.CreateResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	discipline, err := c.svc.CreateDiscipline(&body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := disciplines.CreateResponse{
		Id: discipline.Id,
	}

	return &response, nil
}

func (c *DisciplineController) GetDiscipline(
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	discipline, err := c.svc.GetDisciplineById(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return discipline, nil
}

func (c *DisciplineController) PatchDiscipline(
	ctx fuego.ContextWithBody[disciplines.PatchDiscipline],
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	discipline, err := c.svc.UpdateDisciplineById(id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return discipline, nil
}

func (c *DisciplineController) DeleteDiscipline(
	ctx fuego.ContextNoBody,
) (any, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	// If discipline is deleted it might possible have
	// linked courses that should be detached.

	// TODO: implement course detaching logic

	err = c.svc.DeleteDisciplineById(id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}
