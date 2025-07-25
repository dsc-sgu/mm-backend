package routes

import (
	"net/http"
	"strconv"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// @description Create new course
// @summary Create new course
// @tags courses
// @accept json
// @produce json
// @param request body courses.CreateCourseType true "Course payload"
// @success 201 {object} courses.CourseType
// @failure 400 {object} apierr.ApiError "Invalid JSON"
// @failure 403 {object} apierr.ApiError "No permission"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses/ [POST]
func CreateCourse(
	c *gin.Context,
	sessionRepo session.Repo,
	courseRepo courses.Repo,
	logger *zap.Logger,
) {
	var inputCreateCourse courses.CreateCourseType
	if err := c.ShouldBindBodyWithJSON(&inputCreateCourse); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	// Get user id by
	sessionId, err := c.Cookie(session.COOKIE_NAME)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.CookieNotExists)
		return
	}

	sessionIdUUID, err := uuid.Parse(sessionId)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.CookieNotExists)
		return
	}

	session, err := sessionRepo.GetById(sessionIdUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		return
	}

	createdCourse, err := courseRepo.Create(&inputCreateCourse, session.UserId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusCreated, createdCourse)
}

// @description Get course data
// @summary Get course data
// @tags courses
// @produce json
// @param courseId path int true "Course ID"
// @success 201 {object} courses.CourseType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Course not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses/:id [GET]
func GetCourse(
	c *gin.Context,
	courseRepo courses.Repo,
	logger *zap.Logger,
) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	course, err := courseRepo.GetById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, course)
}

// @description From course list get page of limited number of courses with custom offset
// @summary Get course list page
// @tags courses
// @produce json
// @param limit query int true "Number of courses on the page"
// @param offset query int true "List offset"
// @success 201 {object} courses.CourseType
// @failure 400 {object} apierr.ApiError "Invalid limit or offset"
// @failure 404 {object} apierr.ApiError "Courses not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses [GET]
func GetCourselistPage(
	c *gin.Context,
	courseRepo courses.Repo,
	logger *zap.Logger,
) {
	limit, err := strconv.Atoi(c.Query("limit"))

	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("Invalid limit"))
		return
	}

	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("Invalid offset"))
		return
	}

	coursePage, err := courseRepo.GetCourselistPage(limit, offset)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, coursePage)
}

// @description Change single or multiple parameters of the course
// @summary Modify course
// @tags courses
// @accept json
// @produce json
// @param courseId path int true "Course ID"
// @param request body courses.UpdateCourseType true "Course payload"
// @success 200 {object} courses.CourseType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Parameter not found"
// @failure 404 {object} apierr.ApiError "Course not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses/:id [PATCH]
func PatchCourse(
	c *gin.Context,
	courseRepo courses.Repo,
	logger *zap.Logger,
) {

	var inputUpdateCourse courses.UpdateCourseType
	if err := c.ShouldBindBodyWithJSON(&inputUpdateCourse); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	updatedCourse, err := courseRepo.UpdateById(id, &inputUpdateCourse)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, updatedCourse)
}

// @description Will permanently delete course
// @summary Delete course
// @tags courses
// @produce json
// @param courseId path int true "Course ID"
// @success 204
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Course not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses/:id [DELETE]
func DeleteCourse(
	c *gin.Context,
	courseRepo courses.Repo,
	blockRepo blocks.Repo,
	logger *zap.Logger,
) {

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlockType{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockRepo.UpdateById(block.Id, &updatedBlock)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apierr.InternalServer)
			logger.Error(err.Error())
			return
		}
	}

	err = courseRepo.DeleteById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
