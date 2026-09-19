package git

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// PatternsFileName is the file consumed by the Git pre-receive hook.
const PatternsFileName = ".mm-patterns"

// Limits for archives submitted as task attempts or templates. Submissions
// are programming assignment sources, so these are generous for that use
// case while still bounding the memory and disk a single upload can consume.
const (
	// MaxZipFileCount is the maximum number of regular files a submitted
	// archive may contain.
	MaxZipFileCount = 2000
	// MaxZipFileSize is the maximum decompressed size of a single file in
	// a submitted archive.
	MaxZipFileSize = 10 << 20 // 10 MiB
	// MaxZipTotalSize is the maximum total decompressed size of all files
	// in a submitted archive combined.
	MaxZipTotalSize = 50 << 20 // 50 MiB
	// MaxZipArchiveSize is the maximum size of the archive itself, enforced
	// at the HTTP layer via huma.Operation.MaxBodyBytes (see internal/api.go)
	// rather than here: rejecting an oversized request body is a transport
	// concern, and checking it only after Huma has already buffered the
	// whole body into memory would be too late to matter. It matches
	// MaxZipTotalSize: a compressed archive cannot reasonably need to be
	// larger than the decompressed content budget it is allowed to produce.
	MaxZipArchiveSize = MaxZipTotalSize
)

// RepoID identifies a participant repository for a task group.
type RepoID struct {
	CourseID      uuid.UUID `json:"courseID"      binding:"required"`
	TaskGroupID   uuid.UUID `json:"taskGroupID"   binding:"required"`
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
	FileName    string    `json:"fileName"    binding:"required"`
	FilePath    string    `json:"filePath"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
	Content     []byte    `json:"content"     binding:"required"`
}

func PatternsFilePath(repoPath string) string {
	return filepath.Join(repoPath, PatternsFileName)
}
