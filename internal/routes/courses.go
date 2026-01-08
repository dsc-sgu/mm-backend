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
	courseService *courses.Service
	blockService  *blocks.Service
}

func NewCourseController(
	courseService *courses.Service,
	blockService *blocks.Service,
) *CourseController {
	return &CourseController{
		courseService: courseService,
		blockService:  blockService,
	}
}

func (c *CourseController) CreateCourse(
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

	return c.courseService.CreateCourse(&body, userID)
}

// Other endpoints rewrote using fuego
func (c *CourseController) GetPaginatedCourses(
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

	return c.courseService.GetPaginatedCourses(limit, id)
}

func (c *CourseController) GetCourse(
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{
			Title: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	return c.courseService.GetCourseById(ctx.Context(), id)
}

func (c *CourseController) PatchCourse(
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

	return c.courseService.UpdateCourseById(id, &body)
}

func (c *CourseController) DeleteCourse(
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

	err = c.courseService.DeleteCourseById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	linkedBlocks, err := c.blockService.GetAllBlocksByCourseId(blockId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	for _, block := range linkedBlocks {

		updatedBlock := blocks.UpdateBlock{
			CourseId: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := c.blockService.UpdateBlockById(block.Id, &updatedBlock)
		if err != nil {
			return nil, fuego.InternalServerError{}
		}
	}

	return nil, nil
}
