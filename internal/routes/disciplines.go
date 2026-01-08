package routes

import (
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
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return c.svc.CreateDiscipline(&body)
}

func (c *DisciplineController) GetDiscipline(
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return c.svc.GetDisciplineById(ctx.Context(), id)
}

func (c *DisciplineController) PatchDiscipline(
	ctx fuego.ContextWithBody[disciplines.PatchDiscipline],
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return c.svc.UpdateDisciplineById(id, &body)
}

func (c *DisciplineController) DeleteDiscipline(
	ctx fuego.ContextNoBody,
) (any, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	// If discipline is deleted it might possible have
	// linked courses that should be detached.

	// TODO: implement course detaching logic

	err = c.svc.DeleteDisciplineById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
