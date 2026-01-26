package users

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(user *CreateModel) (*Model, error)
	GetUserByEmail(ctx context.Context, email string) (*Model, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*Model, error)
	DeleteUserByID(id uuid.UUID) error
}
