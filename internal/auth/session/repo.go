package session

import (
	"time"

	"github.com/google/uuid"
)

type Seconds = int

// Session is the database representation of a session.
type Session struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserID    uuid.UUID `json:"userID"`
}

func (session *Session) IsExpired() bool {
	return time.Now().After(session.ExpiresAt)
}

type Repo interface {
	Create(userID uuid.UUID, lifetime Seconds) (*Session, error)
	GetByID(id uuid.UUID) (*Session, error)
	DeleteByID(id uuid.UUID) error
}
