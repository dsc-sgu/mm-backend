package git

import (
	"github.com/charmbracelet/ssh"
	"github.com/dsc-sgu/mm-backend/pkg/git"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"
)

type DBRepo interface {
	AddSshKey(model *SshKey) error
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error
	GetParticipant(fingerprint string) (uuid.UUID, error)
	GetTask(name string) (uuid.UUID, error)
	GetCourse(name string) (uuid.UUID, error)
	CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool
	CheckPasswordAuth(ctx ssh.Context, password string) bool // TODO
	SaveAttempt(repoID RepoID, commitHash string) error
}

type Repo interface {
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error) // TODO
	InitRepo(repoID RepoID) error
	AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel
	RemoveRepo(repoID RepoID) error
	RepoRename(original string, publicKey gossh.PublicKey) (string, error)
	GetRepoID(path string, fingerprint string) (RepoID, error)
}
