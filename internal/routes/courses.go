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
