package users

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(user *CreateUser) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	DeleteUserByID(id uuid.UUID) error
}
