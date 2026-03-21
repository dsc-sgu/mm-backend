package users

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RegularUserRole UserRole = "USER"
	AdminUserRole   UserRole = "ADMIN"
)

// UserEntity is the database representation of a user.
type UserEntity struct {
	ID           uuid.UUID `db:"id"`
	CreatedAt    time.Time `db:"created_at"`
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Patronymic   string    `db:"patronymic"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	AvatarURL    string    `db:"avatar_url"`
	Role         UserRole  `db:"role"`
	PasswordHash []byte    `db:"password_hash"`
	PasswordSalt []byte    `db:"password_salt"`
}

type Repo interface {
	CreateUser(ctx context.Context, user *NewUser) (*UserEntity, error)
	GetUserByEmail(ctx context.Context, email string) (*UserEntity, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*UserEntity, error)
	DeleteUserByID(ctx context.Context, id uuid.UUID) error
}
