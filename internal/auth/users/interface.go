package users

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(u *CreateModel) (*Model, error)
	GetUserByEmail(ctx context.Context, email string) (*Model, error)
	GetUserById(ctx context.Context, id uuid.UUID) (*Model, error)
	DeleteUserById(id uuid.UUID) error
}
