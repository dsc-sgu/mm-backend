package routes

import (
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

type DisciplineController struct {
	service disciplines.Service
}

func NewDisciplineService(repo disciplines.Repo) *DisciplineController {
	return &DisciplineController{
		service: *disciplines.NewService(repo),
	}
}

func (svc *DisciplineController) CreateDiscipline(
	ctx fuego.ContextWithBody[disciplines.CreateDiscipline],
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return svc.service.CreateDiscipline(&body)
}

func (svc *DisciplineController) GetDiscipline(
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return svc.service.GetDisciplineById(ctx.Context(), id)
}

func (svc *DisciplineController) PatchDiscipline(
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

	return svc.service.UpdateDisciplineById(id, &body)
}

func (svc *DisciplineController) DeleteDiscipline(
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

	err = svc.service.DeleteDisciplineById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
