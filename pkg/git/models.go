package git

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	// PatternsFileName is the file consumed by the Git pre-receive hook.
	PatternsFileName = ".mm-patterns"
	// MaxZipFileCount is the maximum number of regular files a submitted
	// archive may contain.
	MaxZipFileCount = 2000
	// MaxZipFileSize is the maximum decompressed size of a single file in
	// a submitted archive.
	MaxZipFileSize = 10 << 20 // 10 MiB
	// MaxZipTotalSize is the maximum total decompressed size of all files
	// in a submitted archive combined. Also enforced at the HTTP layer as
	// the request body size limit (see internal/api.go): a compressed
	// archive cannot reasonably need to be larger than the decompressed
	// content budget it is allowed to produce.
	MaxZipTotalSize = 50 << 20 // 50 MiB
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

func TemplateRepoName(taskGroupID uuid.UUID) string {
	hasher := sha1.New()
	hasher.Write([]byte(taskGroupID.String()))
	return hex.EncodeToString(hasher.Sum(nil)) + ".git"
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

// FileStatus is how a file changed between the two commits Manager.Diff compares.
type FileStatus string

const (
	FileAdded   FileStatus = "added"
	FileDeleted FileStatus = "deleted"
	FileChanged FileStatus = "changed"
)

// ChangedFile is one file's change between the two commits Manager.Diff
// compares. OldText/NewText hold only the removed/added lines (with no
// surrounding context) — not the file's full content on either side. For a
// binary file, Binary is true and both are empty.
type ChangedFile struct {
	Path    string     `json:"path"`
	Status  FileStatus `json:"status"`
	OldText string     `json:"oldText"`
	NewText string     `json:"newText"`
	Binary  bool       `json:"binary"`
}
