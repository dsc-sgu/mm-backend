package attempt

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pingcap/tidb/session"
)

type FileStorageImpl struct {
	s3db *s3.S3
}

func NewFileStorageImpl(sess *session.Session) FileStorage {
	return &FileStorageImpl{
		s3db: s3.New(sess),
	}
}

func (f *FileStorageImpl) StoreFile(fileInfo FileInfo) (FileInfo, error) {
	return fileInfo, nil
}

func (f *FileStorageImpl) GetFile(desc FileDescriptor) (FileInfo, error) {
	return FileInfo{}, nil
}

func (f *FileStorageImpl) RemoveFile(desc FileDescriptor) error {
	return nil
}
