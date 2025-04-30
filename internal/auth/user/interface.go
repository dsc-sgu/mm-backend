package user

import "github.com/google/uuid"

type Repo interface {
	Create(user *CreateModel) (*Model, error)
	GetByEmail(email string) (*Model, error)
	GetById(id uuid.UUID) (*Model, error)
	DeleteById(id uuid.UUID) error
}
