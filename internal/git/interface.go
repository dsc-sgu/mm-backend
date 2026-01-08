package git

import "github.com/google/uuid"

type Repo interface {
	AddSshKey(model *SshKey) error
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error
}
