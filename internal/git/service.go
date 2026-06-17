package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/pkg/git"
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

	return strings.Split(patch.String(), "\n"), nil
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

func (s *Service) AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel {
	if s.db.CheckPublicKeyAuth(nil, pk) {
		return git.ReadWriteAccess
	}
	return git.NoAccess
}

func (s *Service) Push(originalPath string, pk ssh.PublicKey) {
	zap.L().Info("Push hook called", zap.String("path", originalPath))

	fingerprint := gossh.FingerprintSHA256(pk)
	repoID, err := s.GetRepoID(originalPath, fingerprint)
	if err != nil {
		zap.L().Error("Push: get repoID", zap.Error(err))
		return
	}

	shaPath := repoID.IntoPath() + ".git"
	optionsPath := filepath.Join(repoDir, shaPath, "push-options")

	optionsData, err := os.ReadFile(optionsPath)
	if err != nil {
		zap.L().Debug("Push: no options file", zap.Error(err))
		return
	}
	defer os.Remove(optionsPath)

	options := strings.Split(strings.TrimSpace(string(optionsData)), "\n")
	if !hasSubmit(options) {
		return
	}

	pos := parseTaskPosition(options)
	if pos == 0 {
		count, err := s.db.GetTaskCount(context.Background(), repoID.TaskGroupID)
		if err != nil || count != 1 {
			zap.L().Warn("Push: cannot determine task position", zap.Error(err))
			return
		}
		pos = 1
	}

	if pos > 1 {
		ok, err := s.db.HasSubmittedAttempt(context.Background(), repoID.ParticipantID, repoID.TaskGroupID, pos-1)
		if err != nil {
			zap.L().Error("Push: sequential check", zap.Error(err))
			return
		}
		if !ok {
			zap.L().Warn("Push rejected: previous task not completed",
				zap.Int("position", pos))
			return
		}
	}

	taskID, err := s.db.GetTaskIDByPosition(context.Background(), repoID.TaskGroupID, pos)
	if err != nil {
		zap.L().Error("Push: get task by position", zap.Error(err))
		return
	}

	bareRepo, err := gogit.PlainOpen(filepath.Join(repoDir, shaPath))
	if err != nil {
		zap.L().Error("Push: open repo", zap.Error(err))
		return
	}

	head, err := bareRepo.Head()
	if err != nil {
		zap.L().Error("Push: get HEAD", zap.Error(err))
		return
	}

	if err := s.db.SaveAttempt(repoID, taskID, head.Hash().String()); err != nil {
		zap.L().Error("Push: save attempt", zap.Error(err))
	}
}

func hasSubmit(options []string) bool {
	return slices.Contains(options, "submit")
}

func parseTaskPosition(options []string) int {
	for _, opt := range options {
		if strings.HasPrefix(opt, "task=") {
			pos, err := strconv.Atoi(strings.TrimPrefix(opt, "task="))
			if err == nil && pos > 0 {
				return pos
			}
		}
	}
	return 0
}

func (s *Service) Fetch(repo string, pk ssh.PublicKey) {
	zap.L().Info("Fetch hook called", zap.String("repo", repo))
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
	repoName := repoID.IntoPath()
	bareRepoPath := filepath.Join(repoDir, repoName+".git")

	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		if err := s.initRepoWithTemplate(repoID); err != nil {
			return "", fmt.Errorf("init repo: %w", err)
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

	zap.L().Info("attempt pushed",
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
	defer os.RemoveAll(tmpDir)

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

	zap.L().Info("repository initialized from template",
		zap.String("template", templateName),
		zap.String("repo", repoID.IntoPath()),
	)
	return nil
}

func (s *Service) GetCourseIDByTaskGroup(taskGroupID uuid.UUID) (uuid.UUID, error) {
	return s.db.GetCourseIDByTaskGroup(context.Background(), taskGroupID)
}
