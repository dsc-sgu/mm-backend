package tasks

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/git"
)

type Service struct {
	Repo
	gitRepo git.Repo
}

func NewService(repo Repo, gitRepo git.Repo) *Service {
	return &Service{repo, gitRepo}
}

func (s *Service) CreateTaskGroup(ctx context.Context, model *CreateTaskGroup) (*TaskGroup, error) {
	return s.Repo.CreateTaskGroup(ctx, model)
}

func (s *Service) GetTaskGroupByID(ctx context.Context, id uuid.UUID) (*TaskGroup, error) {
	return s.Repo.GetTaskGroupByID(ctx, id)
}

func (s *Service) GetTaskGroupByName(ctx context.Context, name string, courseID uuid.UUID) (*TaskGroup, error) {
	return s.Repo.GetTaskGroupByName(ctx, name, courseID)
}

func (s *Service) UpdateTaskGroup(ctx context.Context, id uuid.UUID, update *UpdateTaskGroup) (*TaskGroup, error) {
	return s.Repo.UpdateTaskGroup(ctx, id, update)
}

func (s *Service) DeleteTaskGroup(ctx context.Context, id uuid.UUID) error {
	return s.Repo.DeleteTaskGroup(ctx, id)
}

func (s *Service) CreateTask(ctx context.Context, model *CreateTask) (*Task, error) {
	return s.Repo.CreateTask(ctx, model)
}

func (s *Service) GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	return s.Repo.GetTaskByID(ctx, taskID)
}

func (s *Service) GetTasks(ctx context.Context, taskGroupID uuid.UUID) ([]*Task, error) {
	return s.Repo.GetTasks(ctx, taskGroupID)
}

func (s *Service) UpdateTask(ctx context.Context, taskID uuid.UUID, update *UpdateTask) (*Task, error) {
	return s.Repo.UpdateTask(ctx, taskID, update)
}

func (s *Service) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.Repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return err
	}

	count, err := s.Repo.GetTaskCount(ctx, task.TaskGroupID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("cannot delete the last task in a group")
	}

	return s.Repo.DeleteTask(ctx, taskID)
}

func (s *Service) GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error) {
	return s.Repo.GetTaskCount(ctx, taskGroupID)
}

func (s *Service) UploadTemplate(ctx context.Context, groupID uuid.UUID, zipData []byte) error {
	tg, err := s.Repo.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if tg == nil {
		return errors.New("task group not found")
	}

	files, err := git.UnzipFiles(zipData)
	if err != nil {
		return err
	}

	return s.gitRepo.UpdateTemplate(tg.ID, files)
}
