package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/go-fuego/fuego"
)

func GetBlock(
	blockRepo blocks.Repo,
	m fuego.ContextWithBody[blocks.BlockID],
) (*blocks.Block, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.GetById(body.ID)
}

func GetAllBlocks(blockRepo blocks.Repo, m fuego.ContextWithBody[courses.CourseID]) ([]*blocks.Block, error) {
	body, err := m.Body()
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

	createdBlock, err := blockRepo.Create(&body)
	if err != nil {
		return nil, err
	}

	return createdBlock, nil
}

func PatchBlock(blockRepo blocks.Repo, m fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
	body, err := m.Body()
	if err != nil {
		return nil, err
	}
	return blockRepo.UpdateById(&body)
}

func DeleteBlock(blockRepo blocks.Repo, m fuego.ContextWithBody[blocks.DeleteBlockFromCourse]) (struct{}, error) {
	body, err := m.Body()
	if err != nil {
		return struct{}{}, err
	}
	err = blockRepo.DeleteById(body.BlockID.ID)
	if err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
}
