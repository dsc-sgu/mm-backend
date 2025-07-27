package disciplines

import "github.com/google/uuid"

type DisciplineID struct {
	ID uuid.UUID `path:"discipline_id" json:"id" binding:"required"`
}

type Discipline struct {
	DisciplineID
	Name string `json:"name" db:"name" binding:"required"`
}

type CreateDiscipline struct {
	DisciplineID
	Name string `json:"name" db:"name" binding:"required"`
}
