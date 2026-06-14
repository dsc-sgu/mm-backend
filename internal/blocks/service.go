package blocks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TODO: Move to config
// Maximum length of position string to trigger lazy position rebalancing
const maxPositionLengthThreshold = 20

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

	block, err := s.Repo.CreateBlock(ctx, model, calculatedPos)
	if err != nil {
		return nil, err
	}

	// Check if rebalance is needed
	if len(calculatedPos) > maxPositionLengthThreshold {
		go s.rebalanceSnapshotPositions(context.Background(), model.SnapshotID)
	}

	return block, nil
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

	// Check if rebalance is needed
	if len(newPosition) > maxPositionLengthThreshold {
		go s.rebalanceSnapshotPositions(context.Background(), snapshotID)
	}

	return nil
}

// rebalanceSnapshotPositions shrinks overgrown position lines,
// distributing them evenly across the available ASCII range
func (s *Service) rebalanceSnapshotPositions(
	ctx context.Context,
	snapshotID uuid.UUID,
) {
	zap.L().
		Info("Starting lazy position rebalancing for snapshot", zap.String("snapshot_id", snapshotID.String()))

	blocksList, err := s.Repo.GetAllBlocksBySnapshotID(ctx, snapshotID)
	if err != nil {
		zap.L().
			Error("rebalance positions: failed to fetch blocks", zap.Error(err))
		return
	}

	// Redistribute positions:
	// first block gets the middle position of the range,
	// every next block gets the position between previous and the end of the range ("~" character)
	currentPrev := ""
	for _, block := range blocksList {
		newPos := CalculateMiddlePosition(currentPrev, "")

		if block.Position != newPos {
			err = s.Repo.UpdateBlockPosition(ctx, block.ID, newPos)
			if err != nil {
				zap.L().Error(
					"rebalance positions: failed to update block position",
					zap.String("block_id", block.ID.String()),
					zap.Error(err),
				)
			}
		}
		currentPrev = newPos
	}
	zap.L().
		Info("Position rebalancing successfully finished", zap.String("snapshot_id", snapshotID.String()))
}

// CalculateMiddlePosition calculates a string that falls lexicographically between two other strings
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
