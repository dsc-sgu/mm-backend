package routes

import (
	"fmt"
	"strconv"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
)

type CourseController struct {
	service courses.Service
}

func NewCourseService(repo courses.Repo) *CourseController {
	return &CourseController{
		service: *courses.NewService(repo),
	}
}

func (svc *CourseController) CreateCourse(
	ctx fuego.ContextWithBody[courses.CreateCourse],
) (*courses.Course, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	return svc.service.CreateCourse(&body, userID)
}

// Other endpoints rewrote using fuego
func (svc *CourseController) GetPaginatedCourses(
	ctx fuego.ContextNoBody,
) ([]courses.Course, error) {
	pathLimit := ctx.QueryParam("limit")
	pathId := ctx.QueryParam("last_id")

	limit, err := strconv.Atoi(pathLimit)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	id, err := uuid.Parse(pathId)
	if err != nil {
		if pathId == "" {
			id = uuid.Nil
		} else {
			return nil, fuego.InternalServerError{Title: fmt.Errorf("parsing UUID: %w", err).Error()}
		}
	}

	return svc.service.GetPaginatedCourses(limit, id)
}

func (svc *CourseController) GetCourse(
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{
			Title: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	return svc.service.GetCourseById(ctx.Context(), id)
}

func (svc *CourseController) PatchCourse(
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

	return svc.service.UpdateCourseById(id, &body)
}

func (svc *CourseController) DeleteCourse(
	blockService *BlockController,
	ctx fuego.ContextNoBody,
) (any, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	blockId, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	err = svc.service.DeleteCourseById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	linkedBlocks, err := blockService.service.GetAllBlocksByCourseId(blockId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := blockService.service.UpdateBlockById(block.Id, &updatedBlock)
		if err != nil {
			return nil, fuego.InternalServerError{}
		}
	}

	return nil, nil
}
