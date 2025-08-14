package attempt

import "github.com/google/uuid"

type RepoManager interface {
	InitRepo(repoID RepoID) error
	RemoveRepo(repoID RepoID) error
	PushCommit(repoID RepoID, fileInfo []FileDescriptor) (Attempt, error)
	GetAttemptData(attemptID uuid.UUID) (AttemptDetails, error)
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
}

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo FileInfo) FileDescriptor
	GetFile(desc FileDescriptor) (FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
