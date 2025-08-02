package user

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	Create(user *CreateModel) (*Model, error)
	GetByEmail(ctx context.Context, email string) (*Model, error)
	GetById(ctx context.Context, id uuid.UUID) (*Model, error)
	DeleteById(id uuid.UUID) error
}
