package routes

import (
	"net/http"

	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// @description Get block data
// @summary Get block data
// @tags blocks
// @produce json
// @param blockId path int true "Block ID"
// @success 201 {object} dto.SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /blocks/:id [GET]
func GetBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	logger *zap.Logger,
) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	block, err := blockRepo.GetById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, block)
}

// @description Get all blocks related to course
// @summary Get All block data
// @tags blocks
// @produce json
// @param blockId path int true "Block ID"
// @success 201 {object} dto.SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /courses/:course_id/blocks [GET]
func GetAllBlocks(
	c *gin.Context,
	blockRepo blocks.Repo,
	logger *zap.Logger,
) {
	courseId, err := uuid.Parse(c.Param("course_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	blockList, err := blockRepo.GetAllBlocksByCourseId(courseId)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, blockList)
}

// @description Add new block
// @summary Add new block
// @tags blocks
// @accept json
// @produce json
// @param courseId path int true "Block ID"
// @param request body dto.SwaggerCreateBlockType true "Block payload"
// @success 201 {object} dto.SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid JSON"
// @failure 403 {object} apierr.ApiError "No permission"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router courses/:course_id/blocks [POST]
func CreateBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	logger *zap.Logger,

) {
	var inputCreateBlock dto.CreateBlockType
	if err := c.ShouldBindBodyWithJSON(&inputCreateBlock); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	courseId, err := uuid.Parse(c.Param("course_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	createdBlock, err := blockRepo.Create(&inputCreateBlock, courseId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusCreated, createdBlock)
}

// @description Change single or multiple parameters of the block
// @summary Modify block
// @tags blocks
// @accept json
// @produce json
// @param blockId path int true "Block ID"
// @param request body dto.SwaggerCreateBlockType true "Block payload"
// @success 200 {object} dto.SwaggerBlockType
// @failure 400 {object} apierr.ApiError "Invalid ID"
// @failure 404 {object} apierr.ApiError "Parameter not found"
// @failure 404 {object} apierr.ApiError "Block not found"
// @failure 500 {object} apierr.ApiError "Internal server error"
// @router /blocks/:id [PATCH]
func PatchBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	logger *zap.Logger,
) {

	var inputUpdateBlock dto.UpdateBlockType
	if err := c.ShouldBindBodyWithJSON(&inputUpdateBlock); err != nil {
		c.JSON(http.StatusBadRequest, apierr.InvalidJSON)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	updatedBlock, err := blockRepo.UpdateById(id, &inputUpdateBlock)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusOK, updatedBlock)
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
// @router /blocks/:id [DELETE]
func DeleteBlock(
	c *gin.Context,
	blockRepo blocks.Repo,
	logger *zap.Logger,
) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, apierr.New("INVALID_ID"))
		return
	}

	err = blockRepo.DeleteById(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, apierr.InternalServer)
		logger.Error(err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
