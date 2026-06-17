package attempt

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"time"

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

	files, err := unzipFiles(zipData)
	if err != nil {
		return "", fmt.Errorf("extract zip: %w", err)
	}

	return s.RepoManager.PushAttempt(repoID, taskID, files)
}

func unzipFiles(data []byte) ([]git.FileInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var files []git.FileInfo
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		files = append(files, git.FileInfo{
			FileName:   f.Name,
			FileSize:   int64(len(content)),
			UploadedAt: time.Now(),
			Content:    content,
		})
	}

	return files, nil
}
