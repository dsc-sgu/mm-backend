package routes

import (
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

type BlockService struct {
	service blocks.Service
}

func NewBlockService(repo blocks.Repo) *BlockService {
	return &BlockService{
		service: *blocks.NewService(repo),
	}
}

func (svc *BlockService) GetBlock(ctx fuego.ContextNoBody) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return svc.service.GetBlock(ctx.Context(), id)
}

func (svc *BlockService) GetAllBlocks(ctx fuego.ContextNoBody) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	return svc.service.GetAllBlocks(id)
}

func (svc *BlockService) CreateBlock(
	ctx fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return svc.service.CreateBlock(ctx.Context(), &body)
}

func (svc *BlockService) PatchBlock(ctx fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return svc.service.UpdateBlock(id, &body)
}

func (svc *BlockService) UnlinkFromCourse(ctx fuego.ContextNoBody) (*blocks.Block, error) {
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

  return svc.service.UnlinkBlock(courseId, blockId)
}

func (svc *BlockService) DeleteBlock(ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, svc.service.DeleteBlock(id)
}
