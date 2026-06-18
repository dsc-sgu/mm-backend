package attempt

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/git"
)

type Service struct {
	RepoManager
	Repo
}

func NewService(repo RepoManager, repo2 Repo) *Service {
	return &Service{repo, repo2}
}

func (s *Service) PushAttempt(courseID, taskGroupID, taskID, participantID uuid.UUID, zipData []byte) (string, error) {
	repoID := git.RepoID{
		CourseID:      courseID,
		TaskGroupID:   taskGroupID,
		ParticipantID: participantID,
	}

	files, err := git.UnzipFiles(zipData)
	if err != nil {
		return "", fmt.Errorf("extract zip: %w", err)
	}

	return s.RepoManager.PushAttempt(repoID, taskID, files)
}
