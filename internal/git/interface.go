package git

import (
	"context"

	"github.com/charmbracelet/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/pkg/git"
)

type DBRepo interface {
	AddSSHKey(model *SSHKey) error
	DeleteSSHKey(ownerID uuid.UUID, fingerprint string) error
	GetParticipant(fingerprint string) (uuid.UUID, error)
	GetCourse(name string) (uuid.UUID, error)
	CheckPublicKeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool
	CheckPasswordAuth(ctx ssh.Context, password string) bool
	SaveAttempt(repoID RepoID, taskID uuid.UUID, commitHash string) error
	GetAttemptCommitInfo(attemptID uuid.UUID) (AttemptCommitInfo, error)

	GetTaskGroupIDByName(ctx context.Context, name string, courseID uuid.UUID) (uuid.UUID, error)
	GetCourseIDByTaskGroup(ctx context.Context, taskGroupID uuid.UUID) (uuid.UUID, error)
	GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error)
	GetTaskIDByPosition(ctx context.Context, taskGroupID uuid.UUID, position int) (uuid.UUID, error)
	HasSubmittedAttempt(ctx context.Context, userID uuid.UUID, taskGroupID uuid.UUID, position int) (bool, error)
}

type Repo interface {
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
	InitRepo(repoID RepoID) error
	AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel
	RemoveRepo(repoID RepoID) error
	RepoRename(original string, pk gossh.PublicKey) (string, error)
	GetRepoID(path string, fingerprint string) (RepoID, error)
	PushAttempt(repoID RepoID, taskID uuid.UUID, files []FileInfo) (string, error)
	UpdateTemplate(taskGroupID uuid.UUID, files []FileInfo) error
}
