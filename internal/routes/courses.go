package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func CreateCourse(
	sessionRepo session.Repo,
	courseRepo courses.Repo,
	logger *zap.Logger,
	m fuego.ContextWithBody[courses.CreateCourseType],
) (*courses.CourseType, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}

	sessionID, err := m.Cookie(session.COOKIE_NAME)
	if err != nil {
		return nil, err
	}

	u, err := uuid.Parse(sessionID.Value)
	if err != nil {
		return nil, err
	}

	session, err := sessionRepo.GetById(u)
	if err != nil {
		logger.Error(err.Error())
		// return nil, err
	}

	createdCourse, err := courseRepo.Create(&body, session.UserId)
	if err != nil {
		return nil, err
	}

	if createdCourse == nil {
		return nil, err
	}

	return createdCourse, nil
}

// Other endpoints rewrote using fuego
func GetPaginatedCourses(courseRepo courses.Repo, logger *zap.Logger, m fuego.ContextWithBody[courses.CoursePagination]) ([]*courses.CourseType, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.GetCourselistPage(body.Limit, body.Offset)
}

func GetCourse(courseRepo courses.Repo, logger *zap.Logger, m fuego.ContextWithBody[courses.CourseID]) (*courses.CourseType, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.GetById(body.ID)
}

func PatchCourse(courseRepo courses.Repo, logger *zap.Logger, m fuego.ContextWithBody[courses.UpdateCourseType]) (*courses.CourseType, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}
	updatedCourse, err := courseRepo.UpdateById(body.ID, &body)
	if err != nil {
		return nil, err
	}
	return updatedCourse, nil
}

func DeleteCourse(courseRepo courses.Repo, blockRepo blocks.Repo, logger *zap.Logger, m fuego.ContextWithBody[courses.CourseID]) (struct{}, error) {
	body, err := m.Body()
	if err != nil {
		return struct{}{}, err
	}
	err = courseRepo.DeleteById(body.ID)
	if err != nil {
		return struct{}{}, err
	}
	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(body.ID)
	if err != nil {
		return struct{}{}, err
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
			return struct{}{}, err
		}
	}

	return struct{}{}, nil
}
