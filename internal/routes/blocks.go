package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
)

func GetBlock(
	blockRepo blocks.Repo,
	ctx fuego.ContextNoBody,
) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	block, err := blockRepo.GetById(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return block, nil
}

func GetAllBlocks(blockRepo blocks.Repo, ctx fuego.ContextNoBody) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}
	blockList, err := blockRepo.GetAllBlocksByCourseId(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return blockList, nil
}

func CreateBlock(
	blockRepo blocks.Repo,
	ctx fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	block, err := blockRepo.Create(ctx.Context(), &body)
	if err != nil {
		return nil, err // fuego.InternalServerError{}
	}

	return block, nil
}

func PatchBlock(blockRepo blocks.Repo, ctx fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	block, err := blockRepo.UpdateById(id, &body)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return block, nil
}

func UnlinkFromCourse(blockRepo blocks.Repo, ctx fuego.ContextNoBody) (*blocks.Block, error) {
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

	block, err := blockRepo.UnlinkFromCourseById(courseId, blockId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return block, nil
}

func DeleteBlock(blockRepo blocks.Repo, ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	err = blockRepo.DeleteById(id)
	if err != nil {
		return nil, fuego.InternalServerError{}
	}

	return nil, nil
}
