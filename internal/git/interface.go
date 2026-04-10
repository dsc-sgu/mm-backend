package git

import (
	"github.com/charmbracelet/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/pkg/git"
)

// functions that use database directly
type DBRepo interface {
	AddSshKey(model *SshKey) error
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error
	GetParticipant(fingerprint string) (uuid.UUID, error)
	AuthRepo(string, ssh.PublicKey) git.AccessLevel
}

// functions that are not used directly by api
type Helpers interface {
	GetCourse(name string) (uuid.UUID, error)
}

// main functions for api
type Repo interface {
	CreateAttemptTag(repoID RepoID, files []FileInfo) error
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
	InitRepo(repoID RepoID) error
	RemoveRepo(repoID RepoID) error
	RepoRename(original string, publicKey gossh.PublicKey) (string, error)
	GetTask(name string) (uuid.UUID, error) // there is no tasks yet
	GetRepoID(path string, fingerprint string) (RepoID, error)
	CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool
	CheckPasswordAuth(ctx ssh.Context, password string) bool
	GitListMiddleware(next ssh.Handler) ssh.Handler
}
