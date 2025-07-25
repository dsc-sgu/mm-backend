package courses

import (
	"time"

	"github.com/google/uuid"
)

type CourseType struct {
	Id           uuid.UUID `json:"id" db:"id" binding:"required"`
	DisciplineId uuid.UUID `json:"disciplineId" db:"discipline_id"`
	OwnerId      uuid.UUID `json:"ownerId" db:"owner_id" binding:"required"`
	Name         string    `json:"name" db:"name" binding:"required"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at" binding:"required"`
}

type CreateCourseType struct {
	DisciplineId uuid.UUID `json:"disciplineId" db:"discipline_id"`
	Name         string    `json:"name" db:"name" binding:"required"`
}

type UpdateCourseType struct {
	OwnerId uuid.UUID `json:"ownerId" db:"owner_id"`
	Name    string    `json:"name" db:"name"`
}
