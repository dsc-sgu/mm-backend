package git

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	fdiff "github.com/go-git/go-git/v6/plumbing/format/diff"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/gobwas/glob"
	"github.com/google/uuid"
)

// Manager owns bare-repository filesystem operations. It has no application-domain dependencies.
type Manager struct {
	RepoDir string
}

func NewManager(repoDir string) *Manager {
	return &Manager{RepoDir: repoDir}
}

func (m *Manager) RepoPath(id RepoID) string { return filepath.Join(m.RepoDir, id.IntoPath()+".git") }

func (m *Manager) InitRepo(id RepoID) error {
	_, err := gogit.PlainInit(m.RepoPath(id), true)
	if err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	return nil
}

func (m *Manager) RemoveRepo(id RepoID) error {
	if err := os.RemoveAll(m.RepoPath(id)); err != nil {
		return fmt.Errorf("remove repo: %w", err)
	}
	return nil
}

func (m *Manager) EnsureRepo(id RepoID) error {
	path := m.RepoPath(id)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return m.initRepoWithTemplate(id)
}

func TemplateRepoName(taskGroupID uuid.UUID) string {
	hasher := sha1.New()
	_, _ = fmt.Fprint(hasher, "template:", taskGroupID.String())
	return hex.EncodeToString(hasher.Sum(nil)) + ".git"
}

func (m *Manager) initRepoWithTemplate(id RepoID) error {
	templatePath := filepath.Join(m.RepoDir, TemplateRepoName(id.TaskGroupID))
	barePath := m.RepoPath(id)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		_, err = gogit.PlainInit(barePath, true)
		return err
	}
	tmp, err := os.MkdirTemp("", "template-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			log.Error("remove temp dir", "error", err, "path", tmp)
		}
	}()
	repo, err := gogit.PlainClone(tmp, &gogit.CloneOptions{URL: templatePath})
	if err != nil {
		return fmt.Errorf("clone template: %w", err)
	}
	if _, err = gogit.PlainInit(barePath, true); err != nil {
		return fmt.Errorf("init student bare: %w", err)
	}
	if _, err = repo.CreateRemote(&config.RemoteConfig{Name: "student", URLs: []string{barePath}}); err != nil {
		return fmt.Errorf("create remote: %w", err)
	}
	if err = repo.Push(&gogit.PushOptions{RemoteName: "student"}); err != nil {
		return fmt.Errorf("push template: %w", err)
	}
	return nil
}

func (m *Manager) UpdateTemplate(taskGroupID uuid.UUID, files []FileInfo) error {
	path := filepath.Join(m.RepoDir, TemplateRepoName(taskGroupID))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err = gogit.PlainInit(path, true); err != nil {
			return fmt.Errorf("init template repo: %w", err)
		}
	}
	_, err := m.commitFiles(path, files, "update template")
	return err
}

func (m *Manager) PushFiles(id RepoID, files []FileInfo) (string, error) {
	if err := m.EnsureRepo(id); err != nil {
		return "", err
	}
	return m.commitFiles(m.RepoPath(id), files, "web attempt")
}

// UnzipFiles extracts regular files from a submitted archive.
// Entry names are sanitized against Zip Slip (path traversal via "../" or
// absolute paths escaping the extraction root once files are later written
// to disk in Manager.commitFiles), and both per-file and total decompressed
// size are capped so a small, highly-compressed archive (a "zip bomb")
// cannot exhaust memory or disk.
func UnzipFiles(data []byte) ([]FileInfo, error) {
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
		// Read fully, failing once more than MaxZipFileSize bytes have been
		// read. Does not trust the zip entry's declared uncompressed size,
		// which an attacker controls independently of the actual compressed
		// data.
		content, err := io.ReadAll(io.LimitReader(rc, MaxZipFileSize+1))
		if err == nil && int64(len(content)) > MaxZipFileSize {
			err = fmt.Errorf("file exceeds the size limit of %d bytes", MaxZipFileSize)
		}
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

func (m *Manager) commitFiles(barePath string, files []FileInfo, message string) (string, error) {
	tmp, err := os.MkdirTemp("", "git-files-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			log.Error("remove temp dir", "error", err, "path", tmp)
		}
	}()
	repo, err := gogit.PlainClone(tmp, &gogit.CloneOptions{URL: barePath})
	if err != nil {
		if err = os.RemoveAll(tmp); err != nil {
			return "", err
		}
		if err = os.MkdirAll(tmp, 0o700); err != nil {
			return "", err
		}
		repo, err = gogit.PlainInit(tmp, false)
		if err != nil {
			return "", err
		}
		if _, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{barePath}}); err != nil {
			return "", err
		}
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	for _, f := range files {
		path := filepath.Join(tmp, f.FileName)
		if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}
		if err = os.WriteFile(path, f.Content, 0o644); err != nil {
			return "", err
		}
		if _, err = wt.Add(f.FileName); err != nil {
			return "", err
		}
	}
	hash, err := wt.Commit(
		message,
		&gogit.CommitOptions{
			Author: &object.Signature{Name: "mm-backend", Email: "mm-backend@mergeminds", When: time.Now()},
		},
	)
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	if err = repo.Push(&gogit.PushOptions{RemoteName: "origin"}); err != nil {
		return "", fmt.Errorf("push: %w", err)
	}
	return hash.String(), nil
}

func (m *Manager) Diff(id RepoID, fromHash, toHash string, patterns []string) ([]string, error) {
	repo, err := gogit.PlainOpen(m.RepoPath(id))
	if err != nil {
		return nil, err
	}
	from, err := repo.CommitObject(plumbing.NewHash(fromHash))
	if err != nil {
		return nil, err
	}
	to, err := repo.CommitObject(plumbing.NewHash(toHash))
	if err != nil {
		return nil, err
	}
	patch, err := from.Patch(to)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return strings.Split(patch.String(), "\n"), nil
	}
	var fps []fdiff.FilePatch
	for _, fp := range patch.FilePatches() {
		fromFile, toFile := fp.Files()
		name := ""
		if toFile != nil {
			name = toFile.Path()
		} else if fromFile != nil {
			name = fromFile.Path()
		}
		if name == "" || MatchesAnyPattern(name, patterns) {
			fps = append(fps, fp)
		}
	}
	buf := &bytes.Buffer{}
	_ = fdiff.NewUnifiedEncoder(buf, fdiff.DefaultContextLines).
		Encode(&filteredPatch{message: patch.Message(), filePatches: fps})
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n"), nil
}

type filteredPatch struct {
	message     string
	filePatches []fdiff.FilePatch
}

func (p *filteredPatch) FilePatches() []fdiff.FilePatch { return p.filePatches }

func (p *filteredPatch) Message() string { return p.message }

// WritePatterns writes the pre-receive hook's ".mm-patterns" file: one
// "<task name>\t<glob>" line per required pattern. A task with no patterns
// still gets a single "<task name>\t" line with an empty glob - the hook
// needs a line for every live task name, not just ones with patterns, to
// tell "no patterns required" apart from "no such task" for an
// "-o submit=<name>" it doesn't recognize.
func (m *Manager) WritePatterns(id RepoID, patterns map[string][]string) error {
	var content strings.Builder
	names := make([]string, 0, len(patterns))
	for name := range patterns {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		filePatterns := patterns[name]
		if len(filePatterns) == 0 {
			_, _ = fmt.Fprintf(&content, "%s\t\n", name)
			continue
		}
		for _, pattern := range filePatterns {
			_, _ = fmt.Fprintf(&content, "%s\t%s\n", name, pattern)
		}
	}
	return os.WriteFile(PatternsFilePath(m.RepoPath(id)), []byte(content.String()), 0o644)
}

// MatchesAnyPattern reports whether name matches any of the given glob
// patterns. Patterns are compiled with no separator characters, so '*' and
// '?' match '/' too — the same rule the pre-receive hook applies via a shell
// case statement (see WritePreReceiveHook), rather than filepath.Match's,
// where '*' stops at '/'. This keeps the accept/reject decision for a
// submission identical whether it arrives over SSH git push or through the
// web zip upload (see the "Mask gate" in internal/attempts/README.md).
func MatchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			continue
		}
		if g.Match(name) {
			return true
		}
	}
	return false
}
