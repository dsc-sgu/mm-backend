package session

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserID    uuid.UUID `json:"userID"`
}

func (session *Session) IsExpired() bool {
	return time.Now().After(session.ExpiresAt)
}
