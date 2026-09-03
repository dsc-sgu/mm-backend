package sshkeys

import (
	"github.com/google/uuid"
)

type Repo interface {
	AddSSHKey(*SSHKey) error
	DeleteSSHKey(ownerID uuid.UUID, fingerprint string) error
	GetParticipant(fingerprint string) (uuid.UUID, error)
}
