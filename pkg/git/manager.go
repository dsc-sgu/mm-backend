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
	"github.com/go-git/go-billy/v6/util"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	fdiff "github.com/go-git/go-git/v6/plumbing/format/diff"
	"github.com/go-git/go-git/v6/plumbing/format/index"
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
	hasher.Write([]byte(taskGroupID.String()))
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
	_, err := m.CommitFiles(path, files, nil, "update template")
	return err
}

func (m *Manager) ListFiles(id RepoID) ([]string, error) {
	repo, err := gogit.PlainOpen(m.RepoPath(id))
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	ref, err := repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("open commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("open tree: %w", err)
	}
	var paths []string
	if err := tree.Files().ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk tree: %w", err)
	}
	return paths, nil
}

// ErrInvalidArchive is returned by UnzipFiles for any way a submitted
// archive can be malformed or violate the limits below — a corrupt zip, a
// Zip Slip or ".git" path, or exceeding a file-count/size limit. Every
// cause is the caller's fault (bad or malicious upload content), never a
// server-side failure, so callers should surface it as a 400, not a 500.
var ErrInvalidArchive = errors.New("invalid archive")

// UnzipFiles extracts regular files from a submitted archive.
// Entry names are sanitized against Zip Slip (path traversal via "../" or
// absolute paths escaping the extraction root once files are later written
// to disk in Manager.commitFiles), and both per-file and total decompressed
// size are capped so a small, highly-compressed archive (a "zip bomb")
// cannot exhaust memory or disk.
func UnzipFiles(data []byte) ([]FileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(reader.File) > MaxZipFileCount {
		return nil, fmt.Errorf(
			"%w: archive contains %d files, exceeding the limit of %d",
			ErrInvalidArchive, len(reader.File), MaxZipFileCount,
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
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidArchive, f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: open %s: %v", ErrInvalidArchive, f.Name, err)
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
			return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidArchive, f.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("%w: close %s: %v", ErrInvalidArchive, f.Name, closeErr)
		}
		totalSize += int64(len(content))
		if totalSize > MaxZipTotalSize {
			return nil, fmt.Errorf(
				"%w: archive exceeds the total decompressed size limit of %d bytes",
				ErrInvalidArchive, MaxZipTotalSize,
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
	// the temporary worktree Manager.CommitFiles clones the repo into,
	// rather than the tracked content — e.g. ".git/hooks/pre-commit" or
	// ".git/config". go-git's Worktree.Add happens to reject such paths
	// today, but only after the file has already been written to disk, so
	// this is enforced here rather than relied on incidentally.
	if slices.Contains(strings.Split(clean, "/"), ".git") {
		return "", errors.New(`path contains a ".git" component`)
	}
	return clean, nil
}

func (m *Manager) CommitFiles(barePath string, files []FileInfo, remove []string, message string) (string, error) {
	tmp, err := os.MkdirTemp("", "git-files-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			log.Error("remove temp dir", "error", err, "path", tmp)
		}
	}()
	repo, err := gogit.PlainClone(tmp, &gogit.CloneOptions{URL: barePath, AllowEmptyRepo: true, Depth: 1})
	if err != nil {
		return "", fmt.Errorf("clone: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	for _, path := range remove {
		if _, err := wt.Remove(path); err != nil && !errors.Is(err, index.ErrEntryNotFound) {
			return "", fmt.Errorf("remove %s: %w", path, err)
		}
	}
	fs := wt.Filesystem()
	for _, f := range files {
		if err := util.WriteFile(fs, f.FileName, f.Content, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", f.FileName, err)
		}
		if err := wt.AddWithOptions(&gogit.AddOptions{Path: f.FileName, SkipStatus: true}); err != nil {
			return "", fmt.Errorf("add %s: %w", f.FileName, err)
		}
	}
	hash, err := wt.Commit(
		message,
		&gogit.CommitOptions{
			Author: &object.Signature{Name: "mm-backend", When: time.Now()},
		},
	)
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	if err := repo.Push(&gogit.PushOptions{RemoteName: "origin"}); err != nil {
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

// CompiledPatterns is a set of glob patterns compiled once via
// CompilePatterns, for matching many names without recompiling on every
// call — useful when a caller checks the same pattern set against a large
// list of paths (e.g. PushAttempt deciding what a resubmission should
// prune).
type CompiledPatterns []*glob.Pattern

// CompilePatterns compiles patterns for repeated matching via
// CompiledPatterns.MatchAny. A pattern that fails to compile is skipped,
// same as MatchesAnyPattern does per call.
func CompilePatterns(patterns []string) CompiledPatterns {
	compiled := make(CompiledPatterns, 0, len(patterns))
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, g)
	}
	return compiled
}

// MatchAny reports whether name matches any of the compiled patterns. See
// MatchesAnyPattern for the matching rules.
func (c CompiledPatterns) MatchAny(name string) bool {
	for _, g := range c {
		if g.Match(name) {
			return true
		}
	}
	return false
}

// MatchesAnyPattern reports whether name matches any of the given glob
// patterns. Patterns are compiled with no separator characters, so '*' and
// '?' match '/' too — the same rule the pre-receive hook applies via a shell
// case statement (see WritePreReceiveHook), rather than filepath.Match's,
// where '*' stops at '/'. This keeps the accept/reject decision for a
// submission identical whether it arrives over SSH git push or through the
// web zip upload (see the "Mask gate" in internal/attempts/README.md).
//
// For matching the same patterns against many names, compile once with
// CompilePatterns instead of calling this repeatedly.
func MatchesAnyPattern(name string, patterns []string) bool {
	return CompilePatterns(patterns).MatchAny(name)
}
