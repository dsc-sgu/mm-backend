package git

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	fdiff "github.com/go-git/go-git/v6/plumbing/format/diff"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"
)

// Manager owns bare-repository filesystem operations. It has no application-domain dependencies.
type Manager struct {
	RepoDir string
	Host    string
	Port    string
}

func NewManager(repoDir, host, port string) *Manager {
	return &Manager{RepoDir: repoDir, Host: host, Port: port}
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

func TemplatePath(taskGroupID uuid.UUID) string {
	hasher := sha1.New()
	_, _ = fmt.Fprint(hasher, "template:", taskGroupID.String())
	return hex.EncodeToString(hasher.Sum(nil)) + ".git"
}

func (m *Manager) initRepoWithTemplate(id RepoID) error {
	templatePath := filepath.Join(m.RepoDir, TemplatePath(id.TaskGroupID))
	barePath := m.RepoPath(id)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		_, err = gogit.PlainInit(barePath, true)
		return err
	}
	tmp, err := os.MkdirTemp("", "template-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
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
	path := filepath.Join(m.RepoDir, TemplatePath(taskGroupID))
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

func (m *Manager) commitFiles(barePath string, files []FileInfo, message string) (string, error) {
	tmp, err := os.MkdirTemp("", "git-files-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
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
	hash, err := wt.Commit(message, &gogit.CommitOptions{Author: &object.Signature{Name: "mm-backend", Email: "mm-backend@mergeminds", When: time.Now()}})
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
	_ = fdiff.NewUnifiedEncoder(buf, fdiff.DefaultContextLines).Encode(&filteredPatch{message: patch.Message(), filePatches: fps})
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n"), nil
}

type filteredPatch struct {
	message     string
	filePatches []fdiff.FilePatch
}

func (p *filteredPatch) FilePatches() []fdiff.FilePatch { return p.filePatches }
func (p *filteredPatch) Message() string                { return p.message }

func (m *Manager) WritePatterns(id RepoID, patterns map[string][]string) error {
	var content strings.Builder
	names := make([]string, 0, len(patterns))
	for name := range patterns {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, pattern := range patterns[name] {
			_, _ = fmt.Fprintf(&content, "%s\t%s\n", name, pattern)
		}
	}
	return os.WriteFile(PatternsFilePath(m.RepoPath(id)), []byte(content.String()), 0o644)
}

func MatchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func (m *Manager) ListMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		if len(sess.Command()) != 0 {
			next(sess)
			return
		}
		dirs, err := os.ReadDir(m.RepoDir)
		if err != nil && err != fs.ErrNotExist {
			log.Error("Invalid repository", "error", err)
		}
		for _, dir := range dirs {
			wish.Println(sess, fmt.Sprintf("git clone ssh://%s/%s", net.JoinHostPort(m.Host, m.Port), dir.Name()))
		}
		next(sess)
	}
}
