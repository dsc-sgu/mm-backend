package routes

import (
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func CreateDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[disciplines.CreateDiscipline],
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	discipline, err := disciplineRepo.CreateDiscipline(&body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return discipline, nil
}

func GetDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {
	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	discipline, err := disciplineRepo.GetDisciplineById(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return discipline, nil
}

func PatchDiscipline(
	disciplineRepo disciplines.Repo,
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

	discipline, err := disciplineRepo.UpdateDisciplineById(id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return discipline, nil
}

func DeleteDiscipline(
	disciplineRepo disciplines.Repo,
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

	err = disciplineRepo.DeleteDisciplineById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
