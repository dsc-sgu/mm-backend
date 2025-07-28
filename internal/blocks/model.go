package blocks

import (
	"encoding/json"

	"github.com/google/uuid"
)

type TextData struct {
	Format string `json:"format"`
	Text   string `json:"text"`
}

type QuizData struct {
	QuestionQuantity int      `json:""`
	Questions        []string `json:""`
	Answers          []string `json:""`
}

type Block struct {
	Id        uuid.UUID       `json:"id"        db:"id"         binding:"required"`
	BlockType string          `json:"blockType" db:"block_type" binding:"required"`
	Data      json.RawMessage `json:"data"      db:"data"       binding:"required"`
	CourseId  uuid.UUID       `json:"courseId"  db:"course_id"  binding:"required"`
	Position  int             `json:"position"  db:"position"   binding:"required"`
}

type CreateBlock struct {
	CourseId  uuid.UUID       `json:"courseId"        db:"course_id"         binding:"required"`
	BlockType string          `json:"blockType" binding:"required"`
	Data      json.RawMessage `json:"data"      binding:"required" swaggertype:"object"`
}

type UpdateBlock struct {
	CourseId uuid.UUID       `json:"courseId"        db:"course_id"         binding:"required"`
	Data     json.RawMessage `json:"data"     swaggertype:"object"`
	Position int             `json:"position"`
}

type DeleteBlockFromCourse struct {
	CourseId uuid.UUID `json:"courseId"        db:"course_id"         binding:"required"`
	Id       uuid.UUID `json:"id"        db:"id"         binding:"required"`
}
