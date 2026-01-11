package routes

import (
	"fmt"

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
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.GetBlockById(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) GetAllBlocks(
	ctx fuego.ContextNoBody,
) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.GetAllBlocksByCourseId(id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) CreateBlock(
	ctx fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.CreateResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	block, err := c.svc.CreateBlock(ctx.Context(), &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := blocks.CreateResponse{
		Id: block.Id,
	}

	return &response, nil
}

func (c *BlockController) PatchBlock(
	ctx fuego.ContextWithBody[blocks.UpdateBlock],
) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	block, err := c.svc.UpdateBlockById(id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) UnlinkFromCourse(
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathBlockId := ctx.PathParam("block_id")

	blockId, err := uuid.Parse(pathBlockId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	pathCourseId := ctx.PathParam("course_id")

	courseId, err := uuid.Parse(pathCourseId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.UnlinkBlockById(courseId, blockId)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) DeleteBlock(ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	err = c.svc.DeleteBlockById(id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}
