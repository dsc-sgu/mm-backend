package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/go-fuego/fuego"
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
	ctx fuego.ContextWithBody[disciplines.DisciplineID],
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return disciplineRepo.GetById(body.ID)
}

func PatchDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[disciplines.PatchDiscipline],
) (*disciplines.Discipline, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	return disciplineRepo.UpdateById(body.ID, &body)
}

func DeleteDiscipline(
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[disciplines.DisciplineID],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	// If discipline is deleted it might possible have
	// linked courses that should be detached.

	// TODO: implement course detaching logic

	return nil, disciplineRepo.DeleteById(body.ID)
}
