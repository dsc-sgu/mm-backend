package attempt

import "github.com/google/uuid"

type RepoManager interface {
	InitRepo(repoID RepoID) error
	RemoveRepo(repoID RepoID) error
	MakeAttempt(repoID RepoID, fileInfo []FileInfo) (attempt Attempt, err error)
	GetAttempts(repoID RepoID) ([]Attempt, error)
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
}

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo FileInfo) (FileDescriptor, error)
	GetFile(desc FileDescriptor) (FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
