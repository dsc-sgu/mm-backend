package blocks

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

func (s *Service) GetBlock(ctx context.Context, id uuid.UUID) (*Block, error) {
	return Repo.GetBlockById(ctx, id)
}

func (s *Service) GetAllBlocks(id uuid.UUID) ([]*Block, error) {
	return Repo.GetAllBlocksByCourseId(id)
}

func (s *Service) CreateBlock(
	ctx context.Context,
	body *CreateBlock,
) (*Block, error) {
	return Repo.CreateBlock(ctx, body)
}

func (s *Service) UpdateBlock(id uuid.UUID, body *UpdateBlock) (*Block, error) {
	return Repo.UpdateBlockById(id, body)
}

func (s *Service) UnlinkBlockById(courseID, blockID uuid.UUID) (*Block, error) {
	return Repo.UnlinkBlockById(courseID, blockID)
}

func (s *Service) DeleteBlockById(id uuid.UUID) error {
	return Repo.DeleteBlockById(id)
}
