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
	ID        uuid.UUID       `json:"id"        db:"id"         binding:"required"`
	BlockType string          `json:"blockType" db:"block_type" binding:"required"`
	Data      json.RawMessage `json:"data"      db:"data"       binding:"required"`
	CourseID  uuid.UUID       `json:"courseID"  db:"course_id"  binding:"required"`
	Position  int             `json:"position"  db:"position"   binding:"required"`
}

type CreateBlock struct {
	CourseID  uuid.UUID       `json:"courseID"  db:"course_id" binding:"required"`
	BlockType string          `json:"blockType"                binding:"required"`
	Data      json.RawMessage `json:"data"                     binding:"required" swaggertype:"object"`
}

type UpdateBlock struct {
	CourseID uuid.UUID       `json:"courseID" db:"course_id" binding:"required"`
	Data     json.RawMessage `json:"data"                                       swaggertype:"object"`
	Position int             `json:"position"`
}

type DeleteBlockFromCourse struct {
	CourseID uuid.UUID `json:"courseID" db:"course_id" binding:"required"`
	ID       uuid.UUID `json:"id"       db:"id"        binding:"required"`
}

type CreateResponse struct {
	ID uuid.UUID `json:"id"`
}
