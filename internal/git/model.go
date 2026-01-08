package git

import (
	"time"

	"github.com/google/uuid"
)

type SshKey struct {
	OwnerId     uuid.UUID `json:"ownerId"     db:"owner_id"    binding:"required"`
	Name        string    `json:"name"        db:"name"        binding:"required"`
	Key         string    `json:"key"         db:"key"         binding:"required"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint" binding:"required"`
	CreatedAt   time.Time `json:"createdAt"   db:"created_at"  binding:"required"`
}

type AddSshKey struct {
	Name string `json:"name" db:"name" binding:"required"`
	Key  string `json:"key"  db:"key"  binding:"required"`
}

type DeleteSshKey struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint" binding:"required"`
}
