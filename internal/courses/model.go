package courses

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	ID           uuid.UUID `json:"id"           db:"id"            binding:"required"`
	DisciplineID uuid.UUID `json:"disciplineID" db:"discipline_id"`
	OwnerID      uuid.UUID `json:"ownerID"      db:"owner_id"      binding:"required"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
	CreatedAt    time.Time `json:"createdAt"    db:"created_at"    binding:"required"`
}

type CreateCourse struct {
	DisciplineID uuid.UUID `json:"disciplineID" db:"discipline_id"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
}

type UpdateCourse struct {
	OwnerID uuid.UUID `json:"ownerID" db:"owner_id"`
	Name    string    `json:"name"    db:"name"`
}

type CoursePagination struct {
	Limit  int       `query:"limit"`
	LastID uuid.UUID `query:"last_id"`
}

type CreateResponse struct {
	ID uuid.UUID `json:"id"`
}
