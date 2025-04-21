package dto

import (
	"encoding/json"

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
