package disciplines

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateDiscipline(model *CreateDiscipline) (*Discipline, error)
	GetDisciplineByID(ctx context.Context, id uuid.UUID) (*Discipline, error)
	UpdateDisciplineByID(
		id uuid.UUID,
		model *PatchDiscipline,
	) (*Discipline, error)
	DeleteDisciplineByID(id uuid.UUID) error
}
