package routes

import (
	"errors"

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
	ctx fuego.ContextWithBody[courses.CreateCourse],
) (*courses.Course, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}

	sessionID, err := ctx.Cookie(session.COOKIE_NAME)
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

	if createdCourse == nil {
		return nil, errors.New("course not created")
	}

	return createdCourse, err
}

// Other endpoints rewrote using fuego
func GetPaginatedCourses(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.CoursePagination],
) ([]*courses.Course, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.GetCourselistPage(body.Limit, body.Offset)
}

func GetCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.CourseID],
) (*courses.Course, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.GetById(body.ID)
}

func PatchCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.UpdateCourse],
) (*courses.Course, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.UpdateById(body.ID, &body)
}

func DeleteCourse(
	courseRepo courses.Repo,
	blockRepo blocks.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.CourseID],
) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	err = courseRepo.DeleteById(body.ID)
	if err != nil {
		return nil, err
	}
	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(body.ID)
	if err != nil {
		return nil, err
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseID: courses.CourseID{
				ID: uuid.Nil,
			},
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockRepo.UpdateById(&updatedBlock)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
