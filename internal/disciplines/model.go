package disciplines

import "github.com/google/uuid"

type DisciplineType struct {
	Id   uuid.UUID `json:"id" db:"id" binding:"required"`
	Name string    `json:"name" db:"name" binding:"required"`
}

type CreateDisciplineType struct {
	Name string `json:"name" db:"name" binding:"required"`
}
