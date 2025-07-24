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
	Id        uuid.UUID       `json:"id" db:"id" binding:"required"`
	BlockType string          `json:"blockType" db:"block_type" binding:"required"`
	Data      json.RawMessage `json:"data" db:"data" binding:"required"`
	CourseId  uuid.UUID       `json:"courseId" db:"course_id" binding:"required"`
	Position  int             `json:"position" db:"position" binding:"required"`
}

// @description For swagger use only. Use BlockType instead
type SwaggerBlockType struct {
	ID        uuid.UUID   `json:"id" binding:"required"`
	BlockType string      `json:"blockType" binding:"required"`
	Data      interface{} `json:"data" binding:"required" swaggertype:"object"`
	CourseId  uuid.UUID   `json:"courseId" binding:"required"`
}

type CreateBlockType struct {
	CourseIDModel
	BlockType string          `json:"blockType" binding:"required"`
	Data      json.RawMessage `json:"data" binding:"required" swaggertype:"object"`
}

// @description For swagger use only. Use CreateBlockType instead
type SwaggerCreateBlockType struct {
	BlockType string      `json:"blockType" binding:"required"`
	Data      interface{} `json:"data" binding:"required"`
}

type BlockIDMODEL struct {
	ID uuid.UUID `path:"block_id" validate:"required"`
}

type CourseIDModel struct {
	ID uuid.UUID `path:"course_id" validate:"required"`
}
type UpdateBlockType struct {
	BlockIDMODEL
	Data     json.RawMessage `json:"data" swaggertype:"object"`
	Position int             `json:"position"`
}

type DeleteBlockFromCourse struct {
	CourseIDModel
	BlockIDMODEL
}

// @description For swagger use only. Use UpdateBlockType instead
type SwaggerUpdateBlockType struct {
	Data     interface{} `json:"data"`
	Position int         `json:"position"`
}
