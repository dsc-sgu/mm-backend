package courses

import (
	"time"

	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/google/uuid"
)

type CourseType struct {
	Id           uuid.UUID
	DisciplineId uuid.UUID
	OwnerId      uuid.UUID
	Name         string
	CreatedAt    time.Time
}

type CreateCourseType struct {
	DisciplineId uuid.UUID
	Name         string
}

type UpdateCourseType struct {
	dto.CourseIDModel
	OwnerId uuid.UUID
	Name    string
}
