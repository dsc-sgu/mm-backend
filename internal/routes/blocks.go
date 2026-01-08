package routes

import (
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

type BlockController struct {
	service blocks.Service
}

func NewBlockService(repo blocks.Repo) *BlockController {
	return &BlockController{
		service: *blocks.NewService(repo),
	}
}

func (svc *BlockController) GetBlock(
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return svc.service.GetBlockById(ctx.Context(), id)
}

func (svc *BlockController) GetAllBlocks(
	ctx fuego.ContextNoBody,
) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return svc.service.GetAllBlocksByCourseId(id)
}

func (svc *BlockController) CreateBlock(
	ctx fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return svc.service.CreateBlock(ctx.Context(), &body)
}

func (svc *BlockController) PatchBlock(
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

	return svc.service.UpdateBlockById(id, &body)
}

func (svc *BlockController) UnlinkFromCourse(
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

	return svc.service.UnlinkBlockById(courseId, blockId)
}

func (svc *BlockController) DeleteBlock(ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, svc.service.DeleteBlockById(id)
}
