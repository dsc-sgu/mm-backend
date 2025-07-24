package routes

import (
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/gin-gonic/gin"
)

func GetBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *dto.BlockIDMODEL,
) (*dto.BlockType, error) {
	return blockRepo.GetById(m.ID)
}

func GetAllBlocks(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *dto.CourseIDModel,
) ([]*dto.BlockType, error) {

	return blockRepo.GetAllBlocksByCourseId(m.ID)

}

func CreateBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *dto.CreateBlockType,
) (*dto.BlockType, error) {

	createdBlock, err := blockRepo.Create(m)
	if err != nil {
		return nil, err
	}

	return createdBlock, nil

}

func PatchBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *dto.UpdateBlockType,
) (*dto.BlockType, error) {

	return blockRepo.UpdateById(m)

}

func DeleteBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	m *dto.BlockIDMODEL,
) (*struct{}, error) {

	return &struct{}{}, blockRepo.DeleteById(m.ID)

}
