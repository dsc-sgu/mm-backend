package routes

import (
	"encoding/json"

	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetBlock(c *gin.Context, m *dto.IdBlockModel) (*dto.BlockType, error) {
	return &dto.BlockType{
		Id:        uuid.New(),
		BlockType: "text",
		Data: json.RawMessage(`{
			format:"markdown",
			text:"Mock text lmao"
	}`),
		CourseId: uuid.New(),
	}, nil
}

func CreateBlock(c *gin.Context, m *dto.CreateBlockType) (*dto.BlockType, error) {
	return &dto.BlockType{
		Id:        uuid.New(),
		BlockType: m.BlockType,
		Data:      m.Data,
		CourseId:  uuid.New(),
	}, nil
}

func PatchBlock(c *gin.Context, m *dto.CreateBlockType) (*dto.BlockType, error) {

	return &dto.BlockType{
		Id:        uuid.New(),
		BlockType: m.BlockType,
		Data:      m.Data,
		CourseId:  uuid.New(),
	}, nil

}

func DeleteBlock(c *gin.Context, m *dto.IdBlockModel) (*struct{}, error) {
	return &struct{}{}, nil
}
