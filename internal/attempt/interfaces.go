package attempt

import "github.com/dsc-sgu/mm-backend/internal/git"

type Repo interface {
	GetAttempts(taskId, participantId uuid.UUID) ([]Attempt, error)
}

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo git.FileInfo) (FileDescriptor, error)
	GetFile(desc FileDescriptor) (git.FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
