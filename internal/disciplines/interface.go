package disciplines

import "github.com/google/uuid"

type Repo interface {
	Create(model *CreateDiscipline) (*Discipline, error)
	GetById(id uuid.UUID) (*Discipline, error)
	UpdateById(id uuid.UUID, model *CreateDiscipline) (*Discipline, error)
	DeleteById(id uuid.UUID) error
}
