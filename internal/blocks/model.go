package blocks

import (
	"encoding/json"

	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/google/uuid"
)

type TextDataType struct {
	Format string `json:"format"`
	Text   string `json:"text"`
}

type QuizDataType struct {
	QuestionQuantity int      `json:""`
	Questions        []string `json:""`
	Answers          []string `json:""`
}

type BlockType struct {
	Id        uuid.UUID       `json:"id"        db:"id"         binding:"required"`
	BlockType string          `json:"blockType" db:"block_type" binding:"required"`
	Data      json.RawMessage `json:"data"      db:"data"       binding:"required"`
	CourseId  uuid.UUID       `json:"courseId"  db:"course_id"  binding:"required"`
	Position  int             `json:"position"  db:"position"   binding:"required"`
}

type CreateBlockType struct {
	courses.CourseID
	BlockType string          `json:"blockType" binding:"required"`
	Data      json.RawMessage `json:"data"      binding:"required" swaggertype:"object"`
}

type BlockID struct {
	ID uuid.UUID `path:"block_id" validate:"required"`
}

type UpdateBlockType struct {
	BlockID
	courses.CourseID
	Data     json.RawMessage `json:"data"     swaggertype:"object"`
	Position int             `json:"position"`
}

type DeleteBlockFromCourse struct {
	courses.CourseID
	BlockID
}
