package routes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TextDataType struct {
	Format string `json:"format" `
	Text   string `json:"text"`
}

type QuizDataType struct {
	QuestionQuantity int      `json:""`
	Questions        []string `json:""`
	Answers          []string `json:""`
}

type BlockType struct {
	Id        uuid.UUID       `json:"id" binding:"required"`
	BlockType string          `json:"blockType" binding:"required"`
	Data      json.RawMessage `json:"data" binding:"required"`
	CourseId  uuid.UUID       `json:"courseId" binding:"required"`
}

// @description For swagger use only. Use BlockType instead
type SwaggerBlockType struct {
	Id        uuid.UUID   `json:"id" binding:"required"`
	BlockType string      `json:"blockType" binding:"required"`
	Data      interface{} `json:"data" binding:"required" swaggertype:"object"`
	CourseId  uuid.UUID   `json:"courseId" binding:"required"`
}

type CreateBlockType struct {
	BlockType string          `json:"blockType" binding:"required"`
	Data      json.RawMessage `json:"data" binding:"required" swaggertype:"object"`
}

// @description For swagger use only. Use CreateBlockType instead
type SwaggerCreateBlockType struct {
	BlockType string      `json:"blockType" binding:"required"`
	Data      interface{} `json:"data" binding:"required"`
}

// @description Get block data
// @summary Get block data
// @tags blocks
// @produce json
// @param blockId path int true "Block ID"
// @success 201 {object} SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /block/:id [GET]
func GetBlock(c *gin.Context) {
	_, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	c.JSON(http.StatusOK, BlockType{
		Id:        uuid.New(),
		BlockType: "text",
		Data: json.RawMessage(`{
			format:"markdown",
			text:"Mock text lmao"
	}`),
		CourseId: uuid.New(),
	})
}

// @description Register a new account
// @summary Register a new account
// @tags blocks
// @accept json
// @produce json
// @param request body SwaggerCreateBlockType true "Block payload"
// @success 201 {object} SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid JSON"
// @failure 403 {object} apierr.ApiError "No permission"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /block [POST]
func CreateBlock(c *gin.Context) {
	var createJson CreateBlockType
	if err := c.ShouldBindBodyWithJSON(&createJson); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}
	c.JSON(http.StatusCreated, BlockType{
		Id:        uuid.New(),
		BlockType: createJson.BlockType,
		Data:      createJson.Data,
		CourseId:  uuid.New(),
	})
}

// @description Change single or multiple parameters of the block
// @summary Modify block
// @tags blocks
// @accept json
// @produce json
// @param blockId path int true "Block ID"
// @param request body SwaggerCreateBlockType true "Block payload"
// @success 200 {object} SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Parameter not found"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /block/:id [PATCH]
func PatchBlock(c *gin.Context) {
	_, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	var req CreateBlockType
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	c.JSON(http.StatusOK, BlockType{
		Id:        uuid.New(),
		BlockType: req.BlockType,
		Data:      req.Data,
		CourseId:  uuid.New(),
	})
}

// @description Will remove block from course but won't delete it from database
// @summary Remove block
// @tags blocks
// @produce json
// @param blockId path int true "Block ID"
// @success 204
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /block/:id [DELETE]
func DeleteBlock(c *gin.Context) {
	_, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
