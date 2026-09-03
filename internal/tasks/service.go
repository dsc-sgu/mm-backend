package tasks

import (
	"context"
	"errors"

	"github.com/google/uuid"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type RepositoryStore interface {
	UpdateTemplate(taskGroupID uuid.UUID, files []pkggit.FileInfo) error
	WritePatterns(repoID pkggit.RepoID, patterns map[string][]string) error
}

type Service struct {
	Repo
	repositories RepositoryStore
}

func NewService(repo Repo, repositories RepositoryStore) *Service {
	return &Service{Repo: repo, repositories: repositories}
}

func (s *Service) UploadTemplate(ctx context.Context, groupID uuid.UUID, zipData []byte) error {
	tg, err := s.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if tg == nil {
		return errors.New("task group not found")
	}

	files, err := pkggit.UnzipFiles(zipData)
	if err != nil {
		return err
	}

	return s.repositories.UpdateTemplate(tg.ID, files)
}

func (s *Service) RefreshRepositoryPatterns(ctx context.Context, repoID pkggit.RepoID) error {
	patterns, err := s.GetTaskPatterns(ctx, repoID.TaskGroupID)
	if err != nil {
		return err
	}
	return s.repositories.WritePatterns(repoID, patterns)
}
