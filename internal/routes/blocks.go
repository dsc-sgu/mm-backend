package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/go-fuego/fuego"
)

func GetBlock(
	blockRepo blocks.Repo,
	ctx fuego.ContextWithBody[blocks.BlockID],
) (*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.GetById(body.ID)
}

func GetAllBlocks(blockRepo blocks.Repo, ctx fuego.ContextWithBody[courses.CourseID]) ([]*blocks.Block, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.GetAllBlocksByCourseId(body.ID)
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
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.UpdateById(&body)
}

func DeleteBlock(blockRepo blocks.Repo, ctx fuego.ContextWithBody[blocks.DeleteBlockFromCourse]) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, err
	}
	return nil, blockRepo.DeleteById(body.BlockID.ID)
}
