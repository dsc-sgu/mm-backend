package attempt

import "github.com/dsc-sgu/mm-backend/internal/git"

type FileDescriptor = string

type FileStorage interface {
	StoreFile(fileInfo git.FileInfo) (FileDescriptor, error)
	GetFile(desc FileDescriptor) (git.FileInfo, error)
	RemoveFile(desc FileDescriptor) error
}
