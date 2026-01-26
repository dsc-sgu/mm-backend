package session

import "github.com/google/uuid"

type Seconds = int

type Repo interface {
	Create(userID uuid.UUID, lifetime Seconds) (*Model, error)
	GetByID(id uuid.UUID) (*Model, error)
	DeleteByID(id uuid.UUID) error
}
