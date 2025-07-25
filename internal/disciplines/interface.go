package disciplines

import "github.com/google/uuid"

type Repo interface {
	Create(model *CreateDisciplineType) (*DisciplineType, error)
	GetById(id uuid.UUID) (*DisciplineType, error)
	UpdateById(id uuid.UUID, model *CreateDisciplineType) (*DisciplineType, error)
	DeleteById(id uuid.UUID) error
}
