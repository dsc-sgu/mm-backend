package disciplines

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	Create(model *CreateDiscipline) (*Discipline, error)
	GetById(ctx context.Context, id uuid.UUID) (*Discipline, error)
	UpdateById(id uuid.UUID, model *PatchDiscipline) (*Discipline, error)
	DeleteById(id uuid.UUID) error
}
