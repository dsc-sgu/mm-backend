package courses

import (
	"time"

	"github.com/google/uuid"
)

type CourseID struct {
	ID uuid.UUID `path:"course_id" validate:"required"`
}

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
	CourseID
	OwnerId uuid.UUID
	Name    string
}

type CoursePagination struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}
