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
	pathID := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.GetBlockByID(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) GetAllBlocks(
	ctx fuego.ContextNoBody,
) ([]*blocks.Block, error) {
	pathID := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.GetAllBlocksByCourseID(ctx.Context(), id)
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
		ID: block.ID,
	}

	return &response, nil
}

func (c *BlockController) PatchBlock(
	ctx fuego.ContextWithBody[blocks.UpdateBlock],
) (*blocks.Block, error) {
	pathID := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	block, err := c.svc.UpdateBlockByID(ctx.Context(), id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) UnlinkFromCourse(
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathBlockID := ctx.PathParam("block_id")

	blockID, err := uuid.Parse(pathBlockID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	pathCourseID := ctx.PathParam("course_id")

	courseID, err := uuid.Parse(pathCourseID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	block, err := c.svc.UnlinkBlockByID(ctx.Context(), courseID, blockID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return block, nil
}

func (c *BlockController) DeleteBlock(ctx fuego.ContextNoBody) (any, error) {
	pathID := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	err = c.svc.DeleteBlockByID(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}
