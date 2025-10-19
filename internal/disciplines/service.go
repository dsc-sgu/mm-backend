package disciplines

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

func (s *Service) CreateDiscipline(
	model *CreateDiscipline,
) (*Discipline, error) {
	return s.repo.CreateDiscipline(model)
}

func (s *Service) GetDiscipline(
	ctx context.Context,
	id uuid.UUID,
) (*Discipline, error) {
	return s.repo.GetDisciplineById(ctx, id)
}

func (s *Service) PatchDiscipline(
	id uuid.UUID,
	model *PatchDiscipline,
) (*Discipline, error) {
	return s.repo.UpdateDisciplineById(id, model)
}

func (s *Service) DeleteDiscipline(id uuid.UUID) error {
	return s.repo.DeleteDisciplineById(id)
}
