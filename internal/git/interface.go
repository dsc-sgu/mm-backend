package git

import "github.com/google/uuid"

type Repo interface {
	AddSshKey(model *SshKey) error
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error
}

type RepoManager interface {
	CreateAttemptTag(repoID RepoID, files []FileInfo) error
	// GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
}
