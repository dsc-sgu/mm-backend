package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/gin-gonic/gin"
)

func GetBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *blocks.BlockID,
) (*blocks.BlockType, error) {
	return blockRepo.GetById(m.ID)
}

func GetAllBlocks(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *courses.CourseID,
) ([]*blocks.BlockType, error) {

	return blockRepo.GetAllBlocksByCourseId(m.ID)

}

func CreateBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *blocks.CreateBlockType,
) (*blocks.BlockType, error) {

	createdBlock, err := blockRepo.Create(m)
	if err != nil {
		return nil, err
	}

	return createdBlock, nil

}

func PatchBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *blocks.UpdateBlockType,
) (*blocks.BlockType, error) {

	return blockRepo.UpdateById(m)

}

func DeleteBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *blocks.BlockID,
) (*struct{}, error) {

	return &struct{}{}, blockRepo.DeleteById(m.ID)
}
