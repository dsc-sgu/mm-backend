package git

import (
	"archive/zip"
	"bytes"
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

// Returns a string with a path of repository on server. It's a
// deterministic local path derived by hashing id, not a
// human-readable name.
func (m *Manager) RepoPath(id RepoID) string { return filepath.Join(m.RepoDir, id.IntoPath()+".git") }

// Checks if repository exists on the server.
func repoExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Checks if user repository exists and creates it otherwise, using template.
func (m *Manager) EnsureRepo(id RepoID) error {
	exists, err := repoExists(m.RepoPath(id))
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return m.initRepoWithTemplate(id)
}

// Creates an user repository based on a existing template repository for exact task group (or single task).
// Falls back to a plain empty repository if no template exists yet for the task group.
func (m *Manager) initRepoWithTemplate(id RepoID) error {
	templatePath := filepath.Join(m.RepoDir, TemplateRepoName(id.TaskGroupID))
	barePath := m.RepoPath(id)
	templateExists, err := repoExists(templatePath)
	if err != nil {
		return err
	}
	if !templateExists {
		_, err := gogit.PlainInit(barePath, true)
		return err
	}
	_, err = gogit.PlainClone(barePath, &gogit.CloneOptions{
		URL:            templatePath,
		Bare:           true,
		AllowEmptyRepo: true,
	})
	if err != nil {
		return fmt.Errorf("clone template: %w", err)
	}
	return nil
}

// Updates existing OR initialises template repository by commiting.
// Unlike attempt uploads, old template files are never removed — only added or overwritten.
func (m *Manager) UpdateTemplate(taskGroupID uuid.UUID, files []FileInfo) error {
	path := filepath.Join(m.RepoDir, TemplateRepoName(taskGroupID))
	exists, err := repoExists(path)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := gogit.PlainInit(path, true); err != nil {
			return fmt.Errorf("init template repo: %w", err)
		}
	}
	_, err = m.CommitFiles(path, files, nil, "update template")
	return err
}

// Returns a list of paths to existing files in user repository.
// Returns nil with no error if the repository has no commits yet.
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

// Unzips an archive of files checking for various types of errors and vulnorabilities.
// Enforces file-count and size limits to guard against zip bombs, without trusting the archive's declared sizes.
func UnzipFiles(data []byte) ([]FileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid archive: %v", err)
	}
	if len(reader.File) > MaxZipFileCount {
		return nil, fmt.Errorf(
			"invalid archive: contains %d files, exceeding the limit of %d",
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
			return nil, fmt.Errorf("invalid archive: %s: %v", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("invalid archive: open %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxZipFileSize+1))
		if err == nil && int64(len(content)) > MaxZipFileSize {
			err = fmt.Errorf("file exceeds the size limit of %d bytes", MaxZipFileSize)
		}
		closeErr := rc.Close()
		if err != nil {
			return nil, fmt.Errorf("invalid archive: read %s: %v", f.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("invalid archive: close %s: %v", f.Name, closeErr)
		}
		totalSize += int64(len(content))
		if totalSize > MaxZipTotalSize {
			return nil, fmt.Errorf(
				"invalid archive: exceeds the total decompressed size limit of %d bytes",
				MaxZipTotalSize,
			)
		}
		files = append(files, FileInfo{
			FileName: name, FileSize: int64(len(content)), UploadedAt: time.Now(), Content: content,
		})
	}
	return files, nil
}

// Checks for errors and attack methods that can be inside file names in archives.
// In particular, a ".git" path component is rejected because CommitFiles later writes
// these paths into a temporary working copy, where such a path would land inside its
// git metadata directory instead of the tracked content.
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
	if slices.Contains(strings.Split(clean, "/"), ".git") {
		return "", errors.New(`path contains a ".git" component`)
	}
	return clean, nil
}

// Commites files to any (user or template) repository. Removes the files, that are passed in remove list within this commit. Also writes a message with commit. Removal happens before the writes.
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

// Returns a difference between to commits from one repository in a form of list of changedFile stuct.
// If include is non-nil, only files for which it returns true are kept. OldText/NewText hold
// only the removed/added lines, not the file's full content; both are empty for binary files.
func (m *Manager) Diff(id RepoID, fromHash, toHash string, include func(path string) bool) ([]ChangedFile, error) {
	repo, err := gogit.PlainOpen(m.RepoPath(id))
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	from, err := commitByHash(repo, fromHash)
	if err != nil {
		return nil, err
	}
	to, err := commitByHash(repo, toHash)
	if err != nil {
		return nil, err
	}
	patch, err := from.Patch(to)
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", fromHash, toHash, err)
	}
	var changed []ChangedFile
	for _, fp := range patch.FilePatches() {
		fromFile, toFile := fp.Files()
		cf := ChangedFile{Binary: fp.IsBinary()}
		switch {
		case fromFile == nil:
			cf.Status, cf.Path = FileAdded, toFile.Path()
		case toFile == nil:
			cf.Status, cf.Path = FileDeleted, fromFile.Path()
		default:
			cf.Status, cf.Path = FileChanged, toFile.Path()
		}
		if include != nil && !include(cf.Path) {
			continue
		}
		if !cf.Binary {
			var oldText, newText strings.Builder
			for _, c := range fp.Chunks() {
				switch c.Type() {
				case fdiff.Delete:
					oldText.WriteString(c.Content())
				case fdiff.Add:
					newText.WriteString(c.Content())
				}
			}
			cf.OldText, cf.NewText = oldText.String(), newText.String()
		}
		changed = append(changed, cf)
	}
	return changed, nil
}

// Helper function to get refference to a commit by its hash.
func commitByHash(repo *gogit.Repository, s string) (*object.Commit, error) {
	h, ok := plumbing.FromHex(s)
	if !ok {
		return nil, fmt.Errorf("invalid commit hash %q", s)
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", s, err)
	}
	return c, nil
}

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
