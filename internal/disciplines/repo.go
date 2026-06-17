package disciplines

import (
	"context"

	"github.com/google/uuid"
)

// Discipline is the database representation of a discipline.
type Discipline struct {
	ID   uuid.UUID `json:"id"   db:"id"   binding:"required"`
	Name string    `json:"name" db:"name" binding:"required"`
}

// CreateDiscipline is the input for creating a discipline, used by both the service and repository layers.
type CreateDiscipline struct {
	Name string `json:"name" db:"name" binding:"required"`
}

// PatchDiscipline is the input for updating a discipline, used by both the service and repository layers.
type PatchDiscipline struct {
	Name string `json:"name" db:"name" binding:"required"`
}

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
