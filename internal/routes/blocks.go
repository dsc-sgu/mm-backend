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
		return nil, err
	}

	return blockRepo.GetById(id)
}

func GetAllBlocks(blockRepo blocks.Repo, ctx fuego.ContextNoBody) ([]*blocks.Block, error) {
	pathId := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}
	blockList, err := blockRepo.GetAllBlocksByCourseId(id)

	if err != nil {
		return nil, err
	}

	return blockList, nil

}

func CreateBlock(
	blockRepo blocks.Repo,
	m fuego.ContextWithBody[blocks.CreateBlock],
) (*blocks.Block, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}

	return blockRepo.Create(&body)
}

func PatchBlock(blockRepo blocks.Repo, ctx fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.UpdateById(id, &body)
}

func DeleteBlock(blockRepo blocks.Repo, ctx fuego.ContextNoBody) (any, error) {
	pathId := ctx.PathParam("block_id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		return nil, err
	}
	return nil, blockRepo.DeleteById(id)
}
