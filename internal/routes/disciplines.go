package routes

import (
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DisciplineService struct {
  service disciplines.Service
}

func NewDisciplineService(repo disciplines.Repo) *DisciplineService {
	return &DisciplineService{
		service: *disciplines.NewService(repo),
	}
}

func (svc *DisciplineService) CreateDiscipline(
	logger *zap.Logger,
	ctx fuego.ContextWithBody[disciplines.CreateDiscipline],
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return svc.service.CreateDiscipline(&body)
}

func (svc *DisciplineService) GetDiscipline(
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return svc.service.GetDiscipline(ctx.Context(), id)
}

func (svc *DisciplineService) PatchDiscipline(
	logger *zap.Logger,
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

	return svc.service.PatchDiscipline(id, &body)
}

func (svc *DisciplineService) DeleteDiscipline(
	logger *zap.Logger,
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

	err = svc.service.DeleteDiscipline(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
