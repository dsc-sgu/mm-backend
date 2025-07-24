package courses

import (
	"time"

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
	OwnerId uuid.UUID
	Name    string
}
