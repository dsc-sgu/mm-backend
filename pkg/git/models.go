package git

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
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
	// MaxZipArchiveSize is the maximum size of the archive itself, checked
	// before it is parsed. It matches MaxZipTotalSize: a compressed archive
	// cannot reasonably need to be larger than the decompressed content
	// budget it is allowed to produce.
	MaxZipArchiveSize = MaxZipTotalSize
)

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
//
// Entry names are sanitized against Zip Slip (path traversal via "../" or
// absolute paths escaping the extraction root once files are later written
// to disk in Manager.commitFiles), and both per-file and total decompressed
// size are capped so a small, highly-compressed archive (a "zip bomb")
// cannot exhaust memory or disk.
func UnzipFiles(data []byte) ([]FileInfo, error) {
	// Checked before zip.NewReader parses the central directory: an archive
	// whose own bytes already exceed the decompressed content budget is not
	// a reasonable submission, and rejecting it here avoids spending time
	// parsing a directory crafted with an excessive number of entries.
	if int64(len(data)) > MaxZipArchiveSize {
		return nil, fmt.Errorf(
			"archive is %d bytes, exceeding the limit of %d",
			len(data), MaxZipArchiveSize,
		)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(reader.File) > MaxZipFileCount {
		return nil, fmt.Errorf(
			"archive contains %d files, exceeding the limit of %d",
			len(reader.File), MaxZipFileCount,
		)
	}

	files := make([]FileInfo, 0, len(reader.File))
	var totalSize int64
	for _, f := range reader.File {
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		name, err := sanitizeZipEntryName(f.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		content, err := readLimited(rc, MaxZipFileSize)
		closeErr := rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", f.Name, closeErr)
		}
		totalSize += int64(len(content))
		if totalSize > MaxZipTotalSize {
			return nil, fmt.Errorf(
				"archive exceeds the total decompressed size limit of %d bytes",
				MaxZipTotalSize,
			)
		}
		files = append(files, FileInfo{
			FileName: name, FileSize: int64(len(content)), UploadedAt: time.Now(), Content: content,
		})
	}
	return files, nil
}

// sanitizeZipEntryName validates a zip entry name and returns it cleaned.
// Zip entries always use "/" as the separator regardless of OS (APPNOTE
// 4.4.17.1), so cleaning is done with the "path" package, not "filepath".
func sanitizeZipEntryName(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty file name")
	}
	if strings.ContainsRune(name, 0) {
		return "", errors.New("file name contains a null byte")
	}
	if strings.Contains(name, "\\") {
		return "", errors.New("file name contains a backslash")
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") ||
		path.IsAbs(clean) {
		return "", errors.New("path escapes the archive root")
	}
	// A ".git" path component would land inside the metadata directory of
	// the temporary worktree Manager.commitFiles clones the repo into,
	// rather than the tracked content — e.g. ".git/hooks/pre-commit" or
	// ".git/config". go-git's Worktree.Add happens to reject such paths
	// today, but only after the file has already been written to disk, so
	// this is enforced here rather than relied on incidentally.
	if slices.Contains(strings.Split(clean, "/"), ".git") {
		return "", errors.New(`path contains a ".git" component`)
	}
	return clean, nil
}

// readLimited reads r fully, failing once more than limit bytes have been
// read. It does not trust the zip entry's declared uncompressed size, which
// an attacker controls independently of the actual compressed data.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("file exceeds the size limit of %d bytes", limit)
	}
	return content, nil
}
