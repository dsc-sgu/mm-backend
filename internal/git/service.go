package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
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
	return s.repo.AddSshKey(&key)
}

func (s *Service) DeleteSshKey(sessionId uuid.UUID, model *DeleteSshKey) error {
	return s.repo.DeleteSshKey(sessionId, model.Fingerprint)
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
