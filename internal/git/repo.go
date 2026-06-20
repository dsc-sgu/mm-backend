package git

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SSHKey struct {
	OwnerID     uuid.UUID `json:"ownerID"     db:"owner_id"    binding:"required"`
	Name        string    `json:"name"        db:"name"        binding:"required"`
	Key         string    `json:"key"         db:"key"         binding:"required"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint" binding:"required"`
	CreatedAt   time.Time `json:"createdAt"   db:"created_at"  binding:"required"`
}

type AddSSHKey struct {
	Name string `json:"name" db:"name" binding:"required"`
	Key  string `json:"key"  db:"key"  binding:"required"`
}

type DeleteSSHKey struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint" binding:"required"`
}

const PatternsFileName = ".patterns"

func PatternsFilePath(repoPath string) string {
	return filepath.Join(repoPath, PatternsFileName)
}

func WritePatternsFile(repoPath string, patterns map[int][]string) error {
	var buf strings.Builder
	positions := make([]int, 0, len(patterns))
	for pos := range patterns {
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	for _, pos := range positions {
		for _, p := range patterns[pos] {
			buf.WriteString(fmt.Sprintf("%d:%s\n", pos, p))
		}
	}
	return os.WriteFile(PatternsFilePath(repoPath), []byte(buf.String()), 0o644)
}

type RepoID struct {
	CourseID      uuid.UUID `json:"courseID"      binding:"required"`
	TaskGroupID   uuid.UUID `json:"taskGroupID"   binding:"required"`
	ParticipantID uuid.UUID `json:"participantID" binding:"required"`
}

func (repoID *RepoID) IntoPath() string {
	hasher := sha1.New()
	data, _ := json.Marshal(repoID)
	hasher.Write(data)
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}

func TemplatePath(taskGroupID uuid.UUID) string {
	hasher := sha1.New()
	_, _ = fmt.Fprint(hasher, "template:", taskGroupID.String())
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}

type FileInfo struct {
	FileName    string    `json:"fileName"    binding:"required"`
	FilePath    string    `json:"filePath"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
	Content     []byte    `json:"content"     binding:"required"`
}

type AttemptCommitInfo struct {
	UserID      uuid.UUID
	TaskID      uuid.UUID
	CommitHash  string
	CourseID    uuid.UUID
	TaskGroupID uuid.UUID
}

func UnzipFiles(data []byte) ([]FileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}

		content, err := io.ReadAll(rc)
		if cerr := rc.Close(); cerr != nil {
			zap.L().Warn("close file reader", zap.String("file", f.Name), zap.Error(cerr))
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		files = append(files, FileInfo{
			FileName:   f.Name,
			FileSize:   int64(len(content)),
			UploadedAt: time.Now(),
			Content:    content,
		})
	}

	return files, nil
}
