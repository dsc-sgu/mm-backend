package routes

import (
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// @description Create new discipline
// @summary Create new discipline
// @tags disciplines
// @accept json
// @produce json
// @param request body disciplines.CreateDisciplineType true "Discipline payload"
// @success 201 {object} disciplines.DisciplineType
// @failure 400 {object} apierr.ApiError "Invalid JSON"
// @failure 403 {object} apierr.ApiError "No permission"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /disciplines/ [POST]
func CreateDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
) {
	var inputCreateDiscipline disciplines.CreateDisciplineType
	if err := c.ShouldBindBodyWithJSON(&inputCreateDiscipline); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	createdDiscipline, err := disciplineRepo.Create(&inputCreateDiscipline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusCreated, createdDiscipline)
}

// @description Get discipline data
// @summary Get discipline data
// @tags disciplines
// @produce json
// @param disciplineId path int true "Discipline ID"
// @success 201 {object} disciplines.DisciplineType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Discipline not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /disciplines/:id [GET]
func GetDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	discipline, err := disciplineRepo.GetById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, discipline)
}

// @description Change single or multiple parameters of the discipline
// @summary Modify discipline
// @tags disciplines
// @accept json
// @produce json
// @param disciplineId path int true "Discipline ID"
// @param request body disciplines.CreateDisciplineType true "Discipline payload"
// @success 200 {object} disciplines.DisciplineType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Parameter not found"
// @failure 404 {object} apierr.ApiError "Discipline not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /disciplines/:id [PATCH]
func PatchDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
) {

	var inputUpdateDiscipline disciplines.CreateDisciplineType
	if err := c.ShouldBindBodyWithJSON(&inputUpdateDiscipline); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	updatedDiscipline, err := disciplineRepo.UpdateById(id, &inputUpdateDiscipline)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, updatedDiscipline)
}

// @description Will permanently delete discipline
// @summary Delete discipline
// @tags disciplines
// @produce json
// @param disciplineId path int true "Discipline ID"
// @success 204
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Discipline not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /disciplines/:id [DELETE]
func DeleteDiscipline(
	c *gin.Context,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
) {

	//If discipline is deleted it might possible have
	//linked courses that should be detached.

	//TODO: implement course detaching logic

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	err = disciplineRepo.DeleteById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
