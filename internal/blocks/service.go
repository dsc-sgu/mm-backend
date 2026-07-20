package blocks

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotNotDraft = errors.New(
		"cannot modify blocks in a non-draft snapshot",
	)
	ErrInvalidBlockForMoveAfter = errors.New(
		"block cannot be moved after itself",
	)
	ErrBlockNotFound      = errors.New("block not found")
	ErrAfterBlockNotFound = errors.New(
		"after_block_id does not exist in this snapshot",
	)
)

type Service struct {
	repo              Repo
	lexoRankThreshold int
}

func NewService(
	repo Repo,
	lexoRankThreshold int,
) *Service {
	return &Service{
		repo:              repo,
		lexoRankThreshold: lexoRankThreshold,
	}
}

// CreateBlock inserts a new block after model.AfterBlockID
func (s *Service) CreateBlock(
	ctx context.Context,
	model *CreateBlock,
	userID, sessionID uuid.UUID,
) (*Block, error) {
	block, err := s.repo.CreateBlock(ctx, model, userID, sessionID)
	if err != nil {
		return nil, err
	}

	// Check if rebalance is needed
	if len(block.Position) > s.lexoRankThreshold {
		go s.rebalanceSnapshotPositions(context.Background(), model.SnapshotID)
	}

	return block, nil
}

// MoveBlock changes block's position so that it comes after afterBlockID
func (s *Service) MoveBlock(
	ctx context.Context,
	blockID, snapshotID, userID, sessionID uuid.UUID,
	afterBlockID *uuid.UUID,
) error {
	if afterBlockID != nil && *afterBlockID == blockID {
		return ErrInvalidBlockForMoveAfter
	}

	newPosition, err := s.repo.MoveBlock(
		ctx,
		blockID,
		snapshotID,
		afterBlockID,
		userID,
		sessionID,
	)
	if err != nil {
		return err
	}

	// Check if rebalance is needed
	if len(newPosition) > s.lexoRankThreshold {
		go s.rebalanceSnapshotPositions(context.Background(), snapshotID)
	}

	return nil
}

func (s *Service) UpdateBlockContent(
	ctx context.Context,
	blockID, snapshotID, userID, sessionID uuid.UUID,
	model *UpdateBlock,
) (*Block, error) {
	return s.repo.UpdateBlockContent(
		ctx,
		blockID,
		snapshotID,
		model,
		userID,
		sessionID,
	)
}

func (s *Service) DeleteBlockByID(
	ctx context.Context,
	blockID, snapshotID, userID, sessionID uuid.UUID,
) error {
	return s.repo.DeleteBlockByID(ctx, blockID, snapshotID, userID, sessionID)
}

func (s *Service) GetBlockByID(
	ctx context.Context,
	blockID uuid.UUID,
) (*Block, error) {
	return s.repo.GetBlockByID(ctx, blockID)
}

func (s *Service) GetAllBlocksBySnapshotID(
	ctx context.Context,
	snapshotID uuid.UUID,
) ([]*Block, error) {
	return s.repo.GetAllBlocksBySnapshotID(ctx, snapshotID)
}

// Fractional indexing
const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	minChar  = '0'
	midChar  = 'V'
)

// rebalanceSnapshotPositions shrinks overgrown position lines,
// distributing them evenly across the available alphabet
func (s *Service) rebalanceSnapshotPositions(
	ctx context.Context,
	snapshotID uuid.UUID,
) {
	zap.L().
		Info("Starting lazy position rebalancing for snapshot", zap.String("snapshot_id", snapshotID.String()))

	blocksList, err := s.repo.GetAllBlocksBySnapshotID(ctx, snapshotID)
	if err != nil {
		zap.L().
			Error("rebalance positions: failed to fetch blocks", zap.Error(err))
		return
	}

	// Redistribute positions:
	// first block gets the middle position of the range,
	// every next block gets the position between previous and the end of the range ("~" character)
	totalBlocks := len(blocksList)
	if totalBlocks == 0 {
		return
	}

	for i, block := range blocksList {
		// Calculate weight from 0.0 to 1.0
		// Add 1 to the value of totalBlocks to leave a gap at the end of the alphabet
		weight := float64(i+1) / float64(totalBlocks+1)

		// Convert weight to position in alphabet
		newPos := weightToPosition(weight, alphabet)

		if block.Position != newPos {
			err = s.repo.UpdateBlockPosition(ctx, block.ID, newPos)
			if err != nil {
				zap.L().
					Error("rebalance positions: failed to update block position", zap.Error(err))
			}
		}
	}
	zap.L().
		Info("Position rebalancing successfully finished", zap.String("snapshot_id", snapshotID.String()))
}

// weightToPosition converts weight from 0.0 to 1.0 to position in 62-char alphabet
func weightToPosition(weight float64, alphabet string) string {
	var result []byte
	base := float64(len(alphabet))

	for range 4 {
		weight *= base
		idx := int(weight)

		result = append(result, alphabet[idx])

		weight -= float64(idx)
		if weight < 1e-9 { // If the remainder is negligible, round and exit
			break
		}
	}
	return string(result)
}

// CalculateMiddlePosition calculates a string that falls lexicographically
// between two other strings
func CalculateMiddlePosition(prev, next string) string {
	// Case 1: both values are empty (blocks table is empty)
	if prev == "" && next == "" {
		return string(midChar)
	}

	// Case 2: insertion at the top
	if prev == "" {
		runes := []rune(next)
		for i := len(runes) - 1; i >= 0; i-- {
			idx := strings.IndexRune(alphabet, runes[i])
			if idx > 0 {
				// If we decrease the character and it becomes equal to minChar (0),
				// we add midChar to give a buffer for future moves (0V).
				if idx-1 == 0 {
					return string(
						runes[:i],
					) + string(
						alphabet[0],
					) + string(
						midChar,
					)
				}
				runes[i] = rune(alphabet[idx-1])
				return string(runes[:i+1])
			}
		}
		return string(minChar) + string(midChar)
	}

	// Case 3: insertion at the bottom
	if next == "" {
		runes := []rune(prev)
		for i := len(runes) - 1; i >= 0; i-- {
			idx := strings.IndexRune(alphabet, runes[i])
			if idx < len(alphabet)-1 {
				runes[i] = rune(alphabet[idx+1])
				return string(runes[:i+1])
			}
		}
		return prev + string(midChar)
	}

	// Case 4: insertion in the middle
	var result strings.Builder

	for i := 0; ; i++ {
		pIdx := 0
		if i < len(prev) {
			pIdx = strings.IndexByte(alphabet, prev[i])
		}

		nIdx := len(alphabet) - 1
		if i < len(next) {
			nIdx = strings.IndexByte(alphabet, next[i])
		}

		// If the last chars of positions are the same, increment the length
		if pIdx == nIdx {
			result.WriteByte(alphabet[pIdx])
			continue
		}

		// If there is a free space between positions, calculate the middle
		if nIdx-pIdx > 1 {
			midIdx := pIdx + (nIdx-pIdx)/2
			result.WriteByte(alphabet[midIdx])
			break
		}

		// If the difference is 1, increment the length
		result.WriteByte(alphabet[pIdx])

		next = ""
	}

	return result.String()
}
