package disciplines

import "github.com/google/uuid"

type Discipline struct {
	Id   uuid.UUID `json:"id"           db:"id"            binding:"required"`
	Name string    `json:"name" db:"name" binding:"required"`
}

type CreateDiscipline struct {
	Name string `json:"name" db:"name" binding:"required"`
}

type PatchDiscipline struct {
	Name string `json:"name" db:"name" binding:"required"`
}
