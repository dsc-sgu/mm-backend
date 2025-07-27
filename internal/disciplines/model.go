package disciplines

import "github.com/google/uuid"

type DisciplineID struct {
	ID uuid.UUID `path:"discipline_id" binding:"required"`
}

type Discipline struct {
	DisciplineID
	Name string `json:"name" db:"name" binding:"required"`
}

type CreateDiscipline struct {
	Name string `json:"name" db:"name" binding:"required"`
}

type PatchDiscipline struct {
	DisciplineID
	Name string `json:"name" db:"name" binding:"required"`
}
