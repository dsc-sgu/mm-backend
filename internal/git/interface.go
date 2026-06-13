package git

import (
	"github.com/charmbracelet/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/pkg/git"
)

type Repo interface {
	AddSshKey(model *SshKey) error                                         // Done
	DeleteSshKey(ownerId uuid.UUID, fingerprint string) error              // Done
	GetParticipant(fingerprint string) (uuid.UUID, error)                  // Done
	GetTask(name string) (uuid.UUID, error)                                // Done
	GetCourse(name string) (uuid.UUID, error)                              // Done
	CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool                // Done
	CheckPasswordAuth(ctx ssh.Context, password string) bool               // TODO
	InitRepo(repoID RepoID) error                                          // Probably Done
	RepoRename(original string, publicKey gossh.PublicKey) (string, error) // Probably Done
	RemoveRepo(repoID RepoID) error                                        // Probably Done
	AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel                // Probably Done
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)            // TODO
	GetRepoID(path string, fingerprint string) (RepoID, error)             // Probably Done
}
