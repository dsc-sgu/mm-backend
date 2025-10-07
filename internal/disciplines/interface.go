package disciplines

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateDiscipline(model *CreateDiscipline) (*Discipline, error)
	GetDisciplineById(ctx context.Context, id uuid.UUID) (*Discipline, error)
	UpdateDisciplineById(
		id uuid.UUID,
		model *PatchDiscipline,
	) (*Discipline, error)
	DeleteDisciplineById(id uuid.UUID) error
}
