package routes

import (
	"errors"
	"strconv"

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

	sessionID, err := ctx.Cookie(session.CookieName)
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
		return nil, err
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
	ctx fuego.ContextNoBody,
) ([]*courses.Course, error) {

	pathLimit := ctx.QueryParam("limit")
	pathOffset := ctx.QueryParam("offset")

	limit, err := strconv.Atoi(pathLimit)
	if err != nil {
		return nil, err
	}

	offset, err := strconv.Atoi(pathOffset)

	if err != nil {
		return nil, err
	}
	return courseRepo.GetPaginatedCourses(limit, offset)
}

func GetCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {

	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}
	return courseRepo.GetById(id)
}

func PatchCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.UpdateCourse],
) (*courses.Course, error) {

	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return courseRepo.UpdateById(id, &body)
}

func DeleteCourse(
	courseRepo courses.Repo,
	blockRepo blocks.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (any, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	err = courseRepo.DeleteById(id)
	if err != nil {
		return nil, err
	}
	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(id)
	if err != nil {
		return nil, err
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockRepo.UpdateById(block.Id, &updatedBlock)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
