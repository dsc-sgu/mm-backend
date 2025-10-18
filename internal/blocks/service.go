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
	return blockRepo.GetBlockById(ctx, id)
}
