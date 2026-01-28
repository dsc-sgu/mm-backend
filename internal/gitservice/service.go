package gitservice

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/attempt"
)

type GitService struct {
	fileStorage attempt.FileStorage
}

func NewGitService() *GitService {
	return &GitService{}
}

func (s *GitService) InitRepo(repoID attempt.RepoID) error {
	repoName := repoID.IntoPath()

	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	repo, err := gogit.PlainInit(repoPath, true)
	if err != nil {
		return err
	}
	fmt.Println(repo)

	return nil
}

func (s *GitService) RemoveRepo(repoID attempt.RepoID) error {
	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoID.IntoPath())
	return os.RemoveAll(repoPath)
}

func (s *GitService) PushAttempt(
	repoID attempt.RepoID,
	files []attempt.FileInfo,
) error {
	repoName := repoID.IntoPath()
	bareRepoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)

	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		if err := s.InitRepo(repoID); err != nil {
			return fmt.Errorf("init repo: %w", err)
		}
	}

	tmpDir, err := os.MkdirTemp("", "attempt-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	repo, err := gogit.PlainClone(tmpDir, &gogit.CloneOptions{
		URL: bareRepoPath,
	})
	if err != nil {
		return fmt.Errorf("clone repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	for _, f := range files {
		storedFile, err := s.fileStorage.GetFile(f.FileName)
		if err != nil {
			return fmt.Errorf("get file %s: %w", f.FileName, err)
		}

		path := filepath.Join(tmpDir, f.FileName)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, storedFile.Content, 0o644); err != nil { // TODO: isn't implemented
			return err
		}
		if _, err := wt.Add(f.FileName); err != nil {
			return err
		}
	}

	commitHash, err := wt.Commit(
		fmt.Sprintf("Attempt %s", 1), // TODO: get attempt ID
		&gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "mm-backend",               // TODO: get name
				Email: "mm-backend@alivetech.org", // TODO: get email
				When:  time.Now(),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := repo.Push(&gogit.PushOptions{}); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	log.Info(
		"attempt pushed",
		"repo", repoName,
		"commit", commitHash.String(),
	)

	return nil
}

func (s *GitService) GetDiff(
	attemptID1, attemptID2 uuid.UUID,
) ([]string, error) {
	// TODO: implement
	return nil, nil
}
