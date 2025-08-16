package attempt

import "github.com/google/uuid"

type RepoManager interface {
	InitRepo(repoID RepoID) error
	RemoveRepo(repoID RepoID) error
	PushAttempt(repoID RepoID, fileInfo []FileInfo) error
	GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
}

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo FileInfo) (FileDescriptor, error)
	GetFile(desc FileDescriptor) (FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
