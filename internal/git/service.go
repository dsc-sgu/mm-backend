package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
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
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

const (
	port    = "2222"
	host    = "localhost"
	repoDir = "repos"
)

type Service struct {
	db DBRepo
}

func NewService(db DBRepo) *Service {
	return &Service{db: db}
}

func (s *Service) RepoRename(original string, pk gossh.PublicKey) (string, error) {
	fingerprint := gossh.FingerprintSHA256(pk)
	repoID, err := s.GetRepoID(original, fingerprint)
	if err != nil {
		return "", err
	}

	repoName := repoID.IntoPath()
	repoPath := repoName + ".git"
	fullPath := filepath.Join(repoDir, repoPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if err := s.initRepoWithTemplate(repoID); err != nil {
			zap.L().Error("RepoRename: init from template", zap.Error(err))
			if err := s.InitRepo(repoID); err != nil {
				return "", err
			}
		}
	}

	// Refresh the masks file on every push so the pre-receive gate always sees
	// the current masks (written unconditionally so removed masks are cleared).
	patterns, err := s.db.GetTaskPatterns(context.Background(), repoID.TaskGroupID)
	if err != nil {
		zap.L().Warn("RepoRename: get patterns", zap.Error(err))
	} else if err := WritePatternsFile(fullPath, patterns); err != nil {
		zap.L().Warn("RepoRename: write patterns file", zap.Error(err))
	}

	return repoPath, nil
}

func (s *Service) AddSSHKey(sessionID uuid.UUID, model *AddSSHKey) error {
	zap.L().Debug("Parsing key", zap.String("key", model.Key))
	authkey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(model.Key))
	if err != nil {
		return errors.New("malformed SSH public key")
	}
	pubkey, err := gossh.ParsePublicKey(authkey.Marshal())
	if err != nil {
		return errors.New("malformed SSH public key")
	}
	fingerprint := gossh.FingerprintSHA256(pubkey)
	key := SSHKey{
		OwnerID:     sessionID,
		Name:        model.Name,
		Key:         string(gossh.MarshalAuthorizedKey(pubkey)),
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}
	return s.db.AddSSHKey(&key)
}

func (s *Service) DeleteSSHKey(sessionID uuid.UUID, model *DeleteSSHKey) error {
	return s.db.DeleteSSHKey(sessionID, model.Fingerprint)
}

func (s *Service) GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error) {
	info1, err := s.db.GetAttemptCommitInfo(attemptID1)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	info2, err := s.db.GetAttemptCommitInfo(attemptID2)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	if info1.UserID != info2.UserID {
		return nil, errors.New("attempts belong to different users")
	}

	repoID := RepoID{
		CourseID:      info1.CourseID,
		TaskGroupID:   info1.TaskGroupID,
		ParticipantID: info1.UserID,
	}

	repoPath := filepath.Join(repoDir, repoID.IntoPath()+".git")

	repo, err := gogit.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo %s: %w", repoPath, err)
	}

	hash1 := plumbing.NewHash(info1.CommitHash)
	hash2 := plumbing.NewHash(info2.CommitHash)

	commit1, err := repo.CommitObject(hash1)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", info1.CommitHash, err)
	}
	commit2, err := repo.CommitObject(hash2)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", info2.CommitHash, err)
	}

	patch, err := commit1.Patch(commit2)
	if err != nil {
		return nil, fmt.Errorf("compute diff: %w", err)
	}

	patterns, _ := s.db.GetTaskPatternsByTaskID(context.Background(), info1.TaskID)
	if len(patterns) > 0 {
		return filterDiffByPatterns(patch, patterns), nil
	}

	return strings.Split(patch.String(), "\n"), nil
}

type filteredPatch struct {
	message     string
	filePatches []fdiff.FilePatch
}

func (p *filteredPatch) FilePatches() []fdiff.FilePatch {
	return p.filePatches
}

func (p *filteredPatch) Message() string {
	return p.message
}

func filterDiffByPatterns(patch *object.Patch, patterns []string) []string {
	var fps []fdiff.FilePatch
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		fileName := ""
		if to != nil {
			fileName = to.Path()
		} else if from != nil {
			fileName = from.Path()
		}
		if fileName == "" || matchesAnyPattern(fileName, patterns) {
			fps = append(fps, fp)
		}
	}

	buf := &bytes.Buffer{}
	encoder := fdiff.NewUnifiedEncoder(buf, fdiff.DefaultContextLines)
	_ = encoder.Encode(&filteredPatch{
		message:     patch.Message(),
		filePatches: fps,
	})
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
}

func matchesAnyPattern(fileName string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, fileName); matched {
			return true
		}
	}
	return false
}

func (s *Service) InitRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := filepath.Join(repoDir, repoName+".git")
	_, err := gogit.PlainInit(repoPath, true)
	if err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	zap.L().Info("repository initialized", zap.String("path", repoPath))
	return nil
}

func (s *Service) RemoveRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := filepath.Join(repoDir, repoName+".git")
	err := os.RemoveAll(repoPath)
	if err != nil {
		return fmt.Errorf("remove repo: %w", err)
	}
	zap.L().Info("repository removed", zap.String("path", repoPath))
	return nil
}

func (s *Service) CheckPublicKeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool {
	return s.db.CheckPublicKeyAuth(ctx, pk)
}

func (s *Service) CheckPasswordAuth(ctx ssh.Context, password string) bool {
	return s.db.CheckPasswordAuth(ctx, password)
}

func (s *Service) AuthRepo(originalPath string, repo string, pk ssh.PublicKey) pkggit.AccessLevel {
	fingerprint := gossh.FingerprintSHA256(pk)

	repoID, err := s.GetRepoID(originalPath, fingerprint)
	if err != nil {
		zap.L().Warn("AuthRepo: resolve repo id", zap.Error(err))
		return pkggit.NoAccess
	}

	member, err := s.db.IsCourseMember(context.Background(), repoID.ParticipantID, repoID.CourseID)
	if err != nil {
		zap.L().Error("AuthRepo: check course membership", zap.Error(err))
		return pkggit.NoAccess
	}
	if !member {
		return pkggit.NoAccess
	}

	return pkggit.ReadWriteAccess
}

func (s *Service) OnPush(originalPath string, repo string, pk ssh.PublicKey) {
	zap.L().Info("Push hook called", zap.String("path", originalPath))

	fingerprint := gossh.FingerprintSHA256(pk)
	repoID, err := s.GetRepoID(originalPath, fingerprint)
	if err != nil {
		zap.L().Error("Push: get repoID", zap.Error(err))
		return
	}

	shaPath := repoID.IntoPath() + ".git"
	repoBase := filepath.Join(repoDir, shaPath)

	optionsPath := filepath.Join(repoBase, "push-options")
	optionsData, err := os.ReadFile(optionsPath)
	if err != nil {
		zap.L().Debug("Push: no options file", zap.Error(err))
		return
	}
	defer func() {
		if err := os.Remove(optionsPath); err != nil {
			zap.L().Warn("remove options file", zap.String("path", optionsPath), zap.Error(err))
		}
	}()

	taskName := parseSubmitOption(strings.Split(strings.TrimSpace(string(optionsData)), "\n"))
	if taskName == "" {
		return
	}

	taskID, err := s.db.GetTaskByName(context.Background(), repoID.TaskGroupID, taskName)
	if err != nil {
		zap.L().Error("Push: get task by name", zap.String("task", taskName), zap.Error(err))
		return
	}

	tagsPath := filepath.Join(repoBase, "push-tags")
	tagsData, err := os.ReadFile(tagsPath)
	if err != nil {
		zap.L().Debug("Push: no tags file", zap.Error(err))
		return
	}
	defer func() {
		if err := os.Remove(tagsPath); err != nil {
			zap.L().Warn("remove push-tags file", zap.String("path", tagsPath), zap.Error(err))
		}
	}()

	for _, hashStr := range strings.Fields(string(tagsData)) {
		if err := s.db.SaveAttempt(repoID, taskID, hashStr); err != nil {
			zap.L().Error("Push: save attempt", zap.Error(err))
		}
	}
}

// parseSubmitOption returns the task name from a "submit=<name>" push option.
func parseSubmitOption(options []string) string {
	for _, opt := range options {
		if v, ok := strings.CutPrefix(strings.TrimSpace(opt), "submit="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Service) OnFetch(originalPath string, repo string, pk ssh.PublicKey) {
	zap.L().Info("Fetch hook called", zap.String("original", originalPath), zap.String("repo", repo))
}

func (s *Service) GetRepoID(path string, fingerprint string) (RepoID, error) {
	if strings.HasPrefix(path, string(os.PathSeparator)) {
		path = path[len(string(os.PathSeparator)):]
	}

	pathList := strings.Split(path, string(os.PathSeparator))
	l := len(pathList)
	if l == 0 {
		return RepoID{}, fmt.Errorf("path is empty")
	}

	last := pathList[l-1]
	suffix := ".git"
	if strings.HasSuffix(last, suffix) {
		pathList[l-1] = last[:len(last)-len(suffix)]
	}

	if len(pathList) < 2 {
		return RepoID{}, fmt.Errorf("invalid path: need course/group_name")
	}

	courseID, err := s.db.GetCourse(pathList[0])
	if err != nil {
		return RepoID{}, err
	}

	taskGroupID, err := s.db.GetTaskGroupIDByName(context.Background(), pathList[1], courseID)
	if err != nil {
		return RepoID{}, err
	}

	participantID, err := s.db.GetParticipant(fingerprint)
	if err != nil {
		return RepoID{}, err
	}

	return RepoID{
		CourseID:      courseID,
		TaskGroupID:   taskGroupID,
		ParticipantID: participantID,
	}, nil
}

func (s *Service) GitListMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		if len(sess.Command()) != 0 {
			next(sess)
			return
		}

		dest, err := os.ReadDir(repoDir)
		if err != nil && err != fs.ErrNotExist {
			log.Error("Invalid repository", "error", err)
		}
		if len(dest) > 0 {
			_, _ = fmt.Fprintf(sess, "\n### Repo Menu ###\n\n")
		}
		for _, dir := range dest {
			wish.Println(sess, fmt.Sprintf("• %s - ", dir.Name()))
			wish.Println(sess, fmt.Sprintf(
				"git clone ssh://%s/%s",
				net.JoinHostPort(host, port),
				dir.Name(),
			))
		}
		wish.Printf(sess, "\n\n### Add some repos! ###\n\n")
		wish.Printf(sess, "> cd some_repo\n")
		wish.Printf(sess, "> git remote add wish_test ssh://%s/some_repo\n", net.JoinHostPort(host, port))
		wish.Printf(sess, "> git push wish_test\n\n\n")
		next(sess)
	}
}

func (s *Service) PushAttempt(repoID RepoID, taskID uuid.UUID, files []FileInfo) (string, error) {
	// Web path bypasses the pre-receive hook (go-git does not run hooks), so the
	// mask gate is duplicated here: reject if no uploaded file matches the task's
	// patterns. Keep in sync with WritePreReceiveHook in pkg/git.
	patterns, err := s.db.GetTaskPatternsByTaskID(context.Background(), taskID)
	if err != nil {
		zap.L().Warn("PushAttempt: get patterns", zap.Error(err))
	} else if len(patterns) > 0 {
		matched := false
		for _, f := range files {
			if matchesAnyPattern(f.FileName, patterns) {
				matched = true
				break
			}
		}
		if !matched {
			return "", fmt.Errorf("no uploaded files match required patterns for this task (%v)", patterns)
		}
	}

	repoName := repoID.IntoPath()
	bareRepoPath := filepath.Join(repoDir, repoName+".git")

	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		if err := s.initRepoWithTemplate(repoID); err != nil {
			return "", fmt.Errorf("init repo: %w", err)
		}
		patterns, err := s.db.GetTaskPatterns(context.Background(), repoID.TaskGroupID)
		if err != nil {
			zap.L().Warn("PushAttempt: get patterns", zap.Error(err))
		} else if len(patterns) > 0 {
			if err := WritePatternsFile(bareRepoPath, patterns); err != nil {
				zap.L().Warn("PushAttempt: write patterns file", zap.Error(err))
			}
		}
	}

	tmpDir, err := os.MkdirTemp("", "attempt-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			zap.L().Error("removing tmp dir", zap.Error(err))
		}
	}()

	repo, err := gogit.PlainClone(tmpDir, &gogit.CloneOptions{
		URL: bareRepoPath,
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		if err := os.MkdirAll(tmpDir, 0o700); err != nil {
			return "", fmt.Errorf("recreate tmp dir: %w", err)
		}

		repo, err = gogit.PlainInit(tmpDir, false)
		if err != nil {
			return "", fmt.Errorf("init work repo: %w", err)
		}

		if _, err := repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{bareRepoPath},
		}); err != nil {
			return "", fmt.Errorf("create remote: %w", err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.FileName)

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", fmt.Errorf("create dirs for %s: %w", f.FileName, err)
		}

		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", f.FileName, err)
		}

		if _, err := wt.Add(f.FileName); err != nil {
			return "", fmt.Errorf("git add %s: %w", f.FileName, err)
		}
	}

	commitHash, err := wt.Commit("web attempt", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "mm-backend",
			Email: "mm-backend@mergeminds",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	if err := repo.Push(&gogit.PushOptions{
		RemoteName: "origin",
	}); err != nil {
		return "", fmt.Errorf("push: %w", err)
	}

	if err := s.db.SaveAttempt(repoID, taskID, commitHash.String()); err != nil {
		return "", fmt.Errorf("save attempt: %w", err)
	}

	zap.L().Info(
		"attempt pushed",
		zap.String("repo", repoName),
		zap.String("commit", commitHash.String()),
	)

	return commitHash.String(), nil
}

func (s *Service) initRepoWithTemplate(repoID RepoID) error {
	templateName := TemplatePath(repoID.TaskGroupID)
	templatePath := filepath.Join(repoDir, templateName+".git")

	bareRepoPath := filepath.Join(repoDir, repoID.IntoPath()+".git")

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		_, err := gogit.PlainInit(bareRepoPath, true)
		return err
	}

	tmpDir, err := os.MkdirTemp("", "template-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			zap.L().Warn("remove temp dir", zap.String("dir", tmpDir), zap.Error(err))
		}
	}()

	repo, err := gogit.PlainClone(tmpDir, &gogit.CloneOptions{
		URL: templatePath,
	})
	if err != nil {
		return fmt.Errorf("clone template: %w", err)
	}

	_, err = gogit.PlainInit(bareRepoPath, true)
	if err != nil {
		return fmt.Errorf("init student bare: %w", err)
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "student",
		URLs: []string{bareRepoPath},
	})
	if err != nil {
		return fmt.Errorf("create remote: %w", err)
	}

	if err := repo.Push(&gogit.PushOptions{
		RemoteName: "student",
	}); err != nil {
		return fmt.Errorf("push template: %w", err)
	}

	zap.L().Info(
		"repository initialized from template",
		zap.String("template", templateName),
		zap.String("repo", repoID.IntoPath()),
	)
	return nil
}

func (s *Service) UpdateTemplate(taskGroupID uuid.UUID, files []FileInfo) error {
	templateName := TemplatePath(taskGroupID)
	templatePath := filepath.Join(repoDir, templateName+".git")

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		if _, err := gogit.PlainInit(templatePath, true); err != nil {
			return fmt.Errorf("init template repo: %w", err)
		}
	}

	tmpDir, err := os.MkdirTemp("", "template-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			zap.L().Warn("remove temp dir", zap.String("dir", tmpDir), zap.Error(err))
		}
	}()

	repo, err := gogit.PlainClone(tmpDir, &gogit.CloneOptions{
		URL: templatePath,
	})
	if err != nil {
		if err := os.RemoveAll(tmpDir); err != nil {
			zap.L().Warn("remove temp dir", zap.String("dir", tmpDir), zap.Error(err))
		}
		if err := os.MkdirAll(tmpDir, 0o700); err != nil {
			return err
		}

		repo, err = gogit.PlainInit(tmpDir, false)
		if err != nil {
			return fmt.Errorf("init work repo: %w", err)
		}

		if _, err := repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{templatePath},
		}); err != nil {
			return fmt.Errorf("create remote: %w", err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.FileName)

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create dirs for %s: %w", f.FileName, err)
		}

		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.FileName, err)
		}

		if _, err := wt.Add(f.FileName); err != nil {
			return fmt.Errorf("git add %s: %w", f.FileName, err)
		}
	}

	if _, err := wt.Commit("update template", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "mm-backend",
			Email: "mm-backend@mergeminds",
			When:  time.Now(),
		},
	}); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := repo.Push(&gogit.PushOptions{
		RemoteName: "origin",
	}); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	return nil
}

func (s *Service) GetCourseIDByTaskGroup(taskGroupID uuid.UUID) (uuid.UUID, error) {
	return s.db.GetCourseIDByTaskGroup(context.Background(), taskGroupID)
}
