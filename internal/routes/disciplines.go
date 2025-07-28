package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
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
		return nil, err
	}

	return disciplineRepo.Create(&body)
}

func GetDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*disciplines.Discipline, error) {

	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}
	return disciplineRepo.GetById(id)
}

func PatchDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[disciplines.PatchDiscipline],
) (*disciplines.Discipline, error) {

	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	return disciplineRepo.UpdateById(id, &body)
}

func DeleteDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (any, error) {

	pathId := ctx.PathParam("discipline_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	// If discipline is deleted it might possible have
	// linked courses that should be detached.

	// TODO: implement course detaching logic

	return nil, disciplineRepo.DeleteById(id)
}
