package session

import "github.com/google/uuid"

type Seconds = int

type Repo interface {
	Create(userID uuid.UUID, lifetime Seconds) (*Session, error)
	GetByID(id uuid.UUID) (*Session, error)
	DeleteByID(id uuid.UUID) error
}
