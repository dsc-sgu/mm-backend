package git

import (
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
	"github.com/dsc-sgu/mm-backend/pkg/git"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

const (
	port    = "2222"
	host    = "localhost"
	repoDir = "repos"
)

// An example git server. This will list all available repos if you ssh
// directly to the server. To test `ssh -p 23233 localhost` once it's running.

type Service struct {
	repo    Repo
	repo_db DBRepo
	helpers Helpers
}

func NewService(repo Repo, repo_db DBRepo, helpers Helpers) *Service {
	return &Service{repo, repo_db, helpers}
}

func (s *Service) RepoRename(original string, publicKey gossh.PublicKey) (string, error) {
	fingerprint := gossh.FingerprintSHA256(publicKey)
	repoID, err := s.repo.GetRepoID(original, fingerprint)
	if err != nil {
		return "", err
	}

	println(repoID.IntoPath())

	repoPath := fmt.Sprintf("%s.git", repoID.IntoPath())

	return repoPath, nil
}

func (s *Service) AddSshKey(sessionId uuid.UUID, model *AddSshKey) error {
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
	key := SshKey{
		OwnerId:     sessionId,
		Name:        model.Name,
		Key:         string(gossh.MarshalAuthorizedKey(pubkey)),
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}
	return s.repo_db.AddSshKey(&key)
}

func (s *Service) DeleteSshKey(sessionId uuid.UUID, model *DeleteSshKey) error {
	return s.repo_db.DeleteSshKey(sessionId, model.Fingerprint)
}

func (s *Service) CreateAttemptTag(repoID RepoID, files []FileInfo) (string, error) {
	repoName := repoID.IntoPath()
	bareRepoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)

	// if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
	// 	if err := s.InitRepo(repoID); err != nil {
	// 		return fmt.Errorf("init repo: %w", err)
	// 	}
	// }

	tmpDir, err := os.MkdirTemp("", "attempt-*")
	if err != nil {
		return "", err
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			zap.L().Error("removing tmp dir: %w", zap.Error(err))
		}
	}()

	repo, err := gogit.PlainClone(tmpDir, &gogit.CloneOptions{
		URL: bareRepoPath,
	})
	if err != nil {
		return "", fmt.Errorf("clone repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.FileName)

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}

		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return "", err
		}

		if _, err := wt.Add(f.FileName); err != nil {
			return "", err
		}
	}

	commitHash, err := wt.Commit(
		fmt.Sprintf("Attempt %d", 1), // TODO: get attempt ID
		&gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "mm-backend",               // TODO: get name
				Email: "mm-backend@alivetech.org", // TODO: get email
				When:  time.Now(),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	tagObj, err := repo.CreateTag(
		fmt.Sprintf("attempt-%d", 1), // TODO: get attempt ID
		commitHash,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("create tag: %w", err)
	}

	if err := repo.Push(&gogit.PushOptions{}); err != nil {
		return "", fmt.Errorf("push: %w", err)
	}

	zap.L().Info("attempt pushed", zap.String("repo", repoName), zap.String("tag", tagObj.Name().String()))

	return tagObj.Name().String(), nil
}

// GetDiff returns the differences between two attempts
func (s *Service) GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error) {
	return []string{"diff placeholder"}, nil
}

// InitRepo creates a new Git repository for the given RepoID
func (s *Service) InitRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	_, err := gogit.PlainInit(repoPath, true)
	if err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	zap.L().Info("repository initialized", zap.String("path", repoPath))
	return nil
}

// RemoveRepo deletes the repository associated with the given RepoID
func (s *Service) RemoveRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	err := os.RemoveAll(repoPath)
	if err != nil {
		return fmt.Errorf("remove repo: %w", err)
	}
	zap.L().Info("repository removed", zap.String("path", repoPath))
	return nil
}

// GetTask retrieves task ID by name (placeholder implementation)
func (s *Service) GetTask(name string) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("task system not implemented yet")
}

// CheckPubkeyAuth verifies SSH public key against database
func (s *Service) CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool {
	return true
}

// CheckPasswordAuth verifies password authentication
func (s *Service) CheckPasswordAuth(ctx ssh.Context, password string) bool {
	return false
}

// AuthRepo checks access level for a repository (for use in middleware)
func (s *Service) AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel {
	if s.CheckPubkeyAuth(nil, pk) {
		return git.ReadWriteAccess
	}
	return git.NoAccess
}

// Push is called after a successful git push operation
func (s *Service) Push(repo string, pk ssh.PublicKey) {
	zap.L().Info("Push hook called", zap.String("repo", repo))
}

// Fetch is called after a successful git fetch operation
func (s *Service) Fetch(repo string, pk ssh.PublicKey) {
	zap.L().Info("Fetch hook called", zap.String("repo", repo))
}

// GetRepoID returns struct RepoID from path and fingerprint
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

	fmt.Println(pathList, len(pathList))

	// TODO: this garbage of a language doesn't have optionals, so I don't know what could go
	// wrong because these are unset. Probably nothing, since all the errors are returned.
	var courseID uuid.UUID
	var taskID uuid.UUID
	if len(pathList) == 1 {
		// TODO: right now, i don't know what will be the interface for getting a task that
		// is course-wide.
		// answer is 0
		return RepoID{}, fmt.Errorf("course-wide tasks are not implemented")
	} else if len(pathList) == 2 {
		var err error
		courseID, err = s.helpers.GetCourse(pathList[0])
		if err != nil {
			return RepoID{}, err
		}
		taskID, err = s.repo.GetTask(pathList[0])
		if err != nil {
			return RepoID{}, err
		}
	}

	participantID, err := s.repo_db.GetParticipant(fingerprint)
	if err != nil {
		return RepoID{}, err
	}

	return RepoID{
		ParticipantID: participantID,
		CourseID:      courseID,
		TaskID:        taskID,
	}, nil
}

// GitListMiddleware provides a list of available repositories when SSHing without a command
func (s *Service) GitListMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		// Git will have a command included so only run this if there are no
		// commands passed to ssh.
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
			wish.Println(
				sess,
				fmt.Sprintf(
					"git clone ssh://%s/%s",
					net.JoinHostPort(host, port),
					dir.Name(),
				),
			)
		}
		wish.Printf(sess, "\n\n### Add some repos! ###\n\n")
		wish.Printf(sess, "> cd some_repo\n")
		wish.Printf(
			sess,
			"> git remote add wish_test ssh://%s/some_repo\n",
			net.JoinHostPort(host, port),
		)
		wish.Printf(sess, "> git push wish_test\n\n\n")
		next(sess)
	}
}
