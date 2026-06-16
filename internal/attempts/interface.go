package attempt

import (
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/git"
)

type Repo interface {
	GetAttempts(taskId, participantId uuid.UUID) ([]Attempt, error)
}

type RepoManager interface {
	GetDiff(id1 uuid.UUID, id2 uuid.UUID) ([]string, error)
	PushAttempt(repoID git.RepoID, files []git.FileInfo) (string, error)
}

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo git.FileInfo) (FileDescriptor, error)
	GetFile(desc FileDescriptor) (git.FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
