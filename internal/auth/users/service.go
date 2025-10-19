package users

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

func (s *Service) GetUserByEmail(ctx context.Context, email string) (*Model, error) {
  return s.repo.GetUserByEmail(ctx, email)
}

func (s *Service) CreateUser(user *CreateModel) (*Model, error) {
  return s.repo.CreateUser(user)
}

func (s *Service) GetUserById(ctx context.Context, id uuid.UUID) (*Model, error) {
  return s.repo.GetUserById(ctx, id)
}

func (s *Service) DeleteUserById(id uuid.UUID) error {
  return s.repo.DeleteUserById(id)
}
