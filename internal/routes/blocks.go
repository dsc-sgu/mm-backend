package routes

import (
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

type BlockController struct {
	svc *blocks.Service
}

func NewBlockController(svc *blocks.Service) *BlockController {
	return &BlockController{
		svc,
	}
}

func (c *BlockController) GetBlock(
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return c.svc.GetBlockById(ctx.Context(), id)
}

func (c *BlockController) GetAllBlocks(
	ctx fuego.ContextNoBody,
) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return c.svc.GetAllBlocksByCourseId(id)
}

func (c *BlockController) CreateBlock(
	ctx fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return c.svc.CreateBlock(ctx.Context(), &body)
}

func (c *BlockController) PatchBlock(
	ctx fuego.ContextWithBody[blocks.UpdateBlock],
) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return c.svc.UpdateBlockById(id, &body)
}

func (c *BlockController) UnlinkFromCourse(
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathBlockId := ctx.PathParam("block_id")

	blockId, err := uuid.Parse(pathBlockId)
	if err != nil {
		return nil, err
	}

	pathCourseId := ctx.PathParam("course_id")

	courseId, err := uuid.Parse(pathCourseId)
	if err != nil {
		return nil, err
	}

	return c.svc.UnlinkBlockById(courseId, blockId)
}

func (c *BlockController) DeleteBlock(ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, c.svc.DeleteBlockById(id)
}
