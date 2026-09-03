package git

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// PatternsFileName is the file consumed by the Git pre-receive hook.
const PatternsFileName = ".mm-patterns"

// RepoID identifies a participant repository for a task group.
type RepoID struct {
	CourseID      uuid.UUID `json:"courseID" binding:"required"`
	TaskGroupID   uuid.UUID `json:"taskGroupID" binding:"required"`
	ParticipantID uuid.UUID `json:"participantID" binding:"required"`
}

func (repoID *RepoID) IntoPath() string {
	hasher := sha1.New()
	data, _ := json.Marshal(repoID)
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

// FileInfo is a file transferred through the Git integration.
type FileInfo struct {
	FileName    string    `json:"fileName" binding:"required"`
	FilePath    string    `json:"filePath" binding:"required"`
	FileSize    int64     `json:"fileSize" binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash" binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt" binding:"required"`
	Content     []byte    `json:"content" binding:"required"`
}

func PatternsFilePath(repoPath string) string {
	return filepath.Join(repoPath, PatternsFileName)
}

// UnzipFiles extracts regular files from a submitted archive.
func UnzipFiles(data []byte) ([]FileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(reader.File))
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", f.Name, closeErr)
		}
		files = append(files, FileInfo{
			FileName: f.Name, FileSize: int64(len(content)), UploadedAt: time.Now(), Content: content,
		})
	}
	return files, nil
}
