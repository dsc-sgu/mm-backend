package routes

import (
	"strconv"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CourseService struct {
  service courses.Service
}

func NewCourseService(repo courses.Repo) *CourseService {
	return &CourseService{
		service: *courses.NewService(repo),
	}
}

func (svc *CourseService) CreateCourse(
	sessionRepo session.Repo,
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

	return svc.service.CreateCourse(&body, session.UserId)
}

// Other endpoints rewrote using fuego
func (svc *CourseService) GetPaginatedCourses(
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

	return svc.service.GetPaginatedCourses(limit, offset)
}

func (svc *CourseService) GetCourse(
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

  return svc.service.GetCourse(ctx.Context(), id)
}

func (svc *CourseService) PatchCourse(
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

	return svc.service.PatchCourse(id, &body)
}

func (svc *CourseService) DeleteCourse(
	blockService *BlockService,
	logger *zap.Logger,
	ctx fuego.ContextNoBody,
) (any, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	err = svc.service.DeleteCourse(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	linkedBlocks, err := blockService.service.GetAllBlocks(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockService.service.UpdateBlock(block.Id, &updatedBlock)
		if err != nil {
			return nil, fuego.InternalServerError{}
		}
	}

	return nil, nil
}
