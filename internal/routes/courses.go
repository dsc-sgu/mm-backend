package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
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
	blockRepo blocks.Repo,
	logger *zap.Logger,
	m *courses.CourseID,
) (*struct{}, error) {

	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(m.ID)
	if err != nil {
		return &struct{}{}, err
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlockType{
			CourseID: courses.CourseID{
				ID: uuid.Nil,
			},
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockRepo.UpdateById(&updatedBlock)
		if err != nil {
			return &struct{}{}, err
		}
	}

	err = courseRepo.DeleteById(m.ID)

	return &struct{}{}, err
}
