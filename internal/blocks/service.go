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
	return s.repo.GetBlockById(ctx, id)
}

func (s *Service) GetAllBlocks(id uuid.UUID) ([]*Block, error) {
	return s.repo.GetAllBlocksByCourseId(id)
}

func (s *Service) CreateBlock(
	ctx context.Context,
	RequestBlock *CreateBlock,
) (*Block, error) {
	return s.repo.CreateBlock(ctx, RequestBlock)
}

func (s *Service) UpdateBlock(id uuid.UUID, body *UpdateBlock) (*Block, error) {
	return s.repo.UpdateBlockById(id, body)
}

func (s *Service) UnlinkBlock(courseID, blockID uuid.UUID) (*Block, error) {
	return s.repo.UnlinkBlockById(courseID, blockID)
}

func (s *Service) DeleteBlock(id uuid.UUID) error {
	return s.repo.DeleteBlockById(id)
}
