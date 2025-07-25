package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CreateDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	m *disciplines.CreateDisciplineType,
) (*disciplines.DisciplineType, error) {

	createdDiscipline, err := disciplineRepo.Create(m)
	if err != nil {
		return nil, err
	}

	return createdDiscipline, nil
}

func GetDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	m *disciplines.DisciplineID,
) (*disciplines.DisciplineType, error) {

	return disciplineRepo.GetById(m.ID)

}

func PatchDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	m *disciplines.CreateDisciplineType,
) (*disciplines.DisciplineType, error) {

	return disciplineRepo.UpdateById(m.ID, m)

}

func DeleteDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	m *disciplines.DisciplineID,
) (*struct{}, error) {

	//If discipline is deleted it might possible have
	//linked courses that should be detached.

	//TODO: implement course detaching logic

	return nil, disciplineRepo.DeleteById(m.ID)

}
