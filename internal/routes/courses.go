package routes

import (
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
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	sessionID, err := ctx.Cookie(session.CookieName)
	if err != nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	u, err := uuid.Parse(sessionID.Value)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	session, err := sessionRepo.GetById(u)
	if err != nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	createdCourse, err := courseRepo.Create(&body, session.UserId)

	if createdCourse == nil {
		return nil, fuego.InternalServerError{Title: "Course wasn't created"}
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
		return nil, fuego.InternalServerError{}
	}

	offset, err := strconv.Atoi(pathOffset)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	courseList, err := courseRepo.GetPaginatedCourses(limit, offset)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return courseList, nil
}

func GetCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	course, err := courseRepo.GetById(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return course, nil
}

func PatchCourse(
	courseRepo courses.Repo,
	logger *zap.Logger,
	ctx fuego.ContextWithBody[courses.UpdateCourse],
) (*courses.Course, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}
	course, err := courseRepo.UpdateById(id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return course, nil
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
		return nil, fuego.InternalServerError{}
	}

	err = courseRepo.DeleteById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	linkedBlocks, err := blockRepo.GetAllBlocksByCourseId(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockRepo.UpdateById(block.Id, &updatedBlock)
		if err != nil {
			return nil, fuego.InternalServerError{}
		}
	}

	return nil, nil
}
