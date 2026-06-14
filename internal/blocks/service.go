package blocks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

// CreateBlock calculates position based on AfterBlockID and initiates block creation
func (s *Service) CreateBlock(
	ctx context.Context,
	model *CreateBlock,
) (*Block, error) {
	leftPos, rightPos, err := s.GetPositionsForMove(
		ctx,
		model.SnapshotID,
		model.AfterBlockID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"service create block: resolve positions: %w",
			err,
		)
	}

	calculatedPos := CalculateMiddlePosition(leftPos, rightPos)

	return s.Repo.CreateBlock(ctx, model, calculatedPos)
}

// MoveBlock calculates new block position and updates it in DB
func (s *Service) MoveBlock(
	ctx context.Context,
	blockID uuid.UUID,
	snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) error {
	leftPos, rightPos, err := s.GetPositionsForMove(
		ctx,
		snapshotID,
		afterBlockID,
	)
	if err != nil {
		return fmt.Errorf("service move block: get positions: %w", err)
	}

	newPosition := CalculateMiddlePosition(leftPos, rightPos)

	err = s.UpdateBlockPosition(ctx, blockID, newPosition)
	if err != nil {
		return fmt.Errorf("service move block: save position: %w", err)
	}

	return nil
}

// CalculateMiddlePosition calculates a string that falls lexicographically between two other strings.
func CalculateMiddlePosition(prev, next string) string {
	if prev == "" {
		prev = " "
	}
	if next == "" {
		next = "~"
	}

	var result []byte
	i := 0

	for {
		var p byte = ' '
		if i < len(prev) {
			p = prev[i]
		}

		var n byte = '~'
		if i < len(next) {
			n = next[i]
		}

		if p >= n {
			// This case should ideally not happen if inputs are always ordered.
			// If it does, we append the smaller character and continue,
			// effectively trying to find a difference in the next character.
			result = append(result, p)
			i++
			continue
		}

		if n-p > 1 {
			mid := p + (n-p)/2
			result = append(result, mid)
			break
		}

		result = append(result, p)
		i++
	}

	return string(result)
}
