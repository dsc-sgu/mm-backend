package courses

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	Id           int       `json:"id"           db:"id"            binding:"required"`
	DisciplineId uuid.UUID `json:"disciplineId" db:"discipline_id"`
	OwnerId      uuid.UUID `json:"ownerId"      db:"owner_id"      binding:"required"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
	CreatedAt    time.Time `json:"createdAt"    db:"created_at"    binding:"required"`
}

type CreateCourse struct {
	DisciplineId uuid.UUID `json:"disciplineId" db:"discipline_id"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
}

type UpdateCourse struct {
	OwnerId uuid.UUID `json:"ownerId" db:"owner_id"`
	Name    string    `json:"name"    db:"name"`
}

type CoursePagination struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}
