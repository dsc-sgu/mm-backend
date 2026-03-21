package git

import (
	"time"

	"github.com/google/uuid"
)

// SshKey is the database representation of an SSH key.
type SshKey struct {
	OwnerId     uuid.UUID `json:"ownerId"     db:"owner_id"    binding:"required"`
	Name        string    `json:"name"        db:"name"        binding:"required"`
	Key         string    `json:"key"         db:"key"         binding:"required"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint" binding:"required"`
	CreatedAt   time.Time `json:"createdAt"   db:"created_at"  binding:"required"`
}

// AddSshKey is the input for adding an SSH key, used by both the service and repository layers.
type AddSshKey struct {
	Name string `json:"name" db:"name" binding:"required"`
	Key  string `json:"key"  db:"key"  binding:"required"`
}

// DeleteSshKey is the input for deleting an SSH key, used by both the service and repository layers.
type DeleteSshKey struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint" binding:"required"`
}

type Repo interface {
	AddSshKey(model *SshKey) error
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error
}
