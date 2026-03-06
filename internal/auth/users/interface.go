package users

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(cxt context.Context, user *CreateUser) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	DeleteUserByID(ctx context.Context, id uuid.UUID) error
}
