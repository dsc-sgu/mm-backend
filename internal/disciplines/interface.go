package disciplines

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateDiscipline(
		ctx context.Context,
		model *CreateDiscipline,
	) (*Discipline, error)
	GetDisciplineByID(ctx context.Context, id uuid.UUID) (*Discipline, error)
	UpdateDisciplineByID(
		ctx context.Context,
		id uuid.UUID,
		model *PatchDiscipline,
	) (*Discipline, error)
	DeleteDisciplineByID(ctx context.Context, id uuid.UUID) error
}
