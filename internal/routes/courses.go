package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func CreateCourse(
	c *gin.Context,
	sessionRepo session.Repo,
	courseRepo courses.Repo,
	logger *zap.Logger,
	m *courses.CreateCourseType,
) (*courses.CourseType, error) {
	// Get user id by
	sessionID, err := c.Cookie(session.COOKIE_NAME)
	if err != nil {
		return nil, err
	}

	u, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}

	session, err := sessionRepo.GetById(u)
	if err != nil {
		return nil, err
	}

	createdCourse, err := courseRepo.Create(m, session.UserId)
	if err != nil {
		return nil, err
	}

	return createdCourse, nil
}

func GetCourse(
	c *gin.Context,
	courseRepo courses.Repo,
	logger *zap.Logger,
	m *courses.CourseID,
) (*courses.CourseType, error) {

	course, err := courseRepo.GetById(m.ID)

	if err != nil {
		return nil, err
	}

	return course, nil
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
	m *courses.CoursePagination,
) ([]*courses.CourseType, error) {

	coursePage, err := courseRepo.GetCourselistPage(m.Limit, m.Offset)

	if err != nil {
		return nil, err
	}

	return coursePage, nil
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
	m *courses.UpdateCourseType,
) (*courses.CourseType, error) {

	updatedCourse, err := courseRepo.UpdateById(m.ID, m)

	if err != nil {
		return nil, err
	}

	return updatedCourse, nil
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
	logger *zap.Logger,
	m *courses.CourseID,
) (*struct{}, error) {

	//If course is deleted it might possible have
	//linked blocks that should be detached.

	//TODO: implement block detaching logic

	err := courseRepo.DeleteById(m.ID)

	return &struct{}{}, err
}
