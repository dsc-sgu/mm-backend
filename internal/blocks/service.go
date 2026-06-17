package blocks

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrSnapshotNotDraft = errors.New(
		"cannot modify blocks in a non-draft snapshot",
	)
	ErrInvalidBlockForMoveAfter = errors.New(
		"block cannot be moved after itself",
	)
)

type Service struct {
	repo              Repo
	snapshotsService  *snapshots.Service
	locksService      *locks.Service
	lexoRankThreshold int
}

func NewService(
	repo Repo,
	snapshotsService *snapshots.Service,
	locksService *locks.Service,
	lexoRankThreshold int,
) *Service {
	return &Service{
		repo:              repo,
		snapshotsService:  snapshotsService,
		locksService:      locksService,
		lexoRankThreshold: lexoRankThreshold,
	}
}

// validateLock checks if the user has a valid lock on the course
func (s *Service) validateLock(
	ctx context.Context,
	snapshotID, userID, sessionID uuid.UUID,
) error {
	snapshot, err := s.snapshotsService.GetSnapshotByID(ctx, snapshotID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return ErrSnapshotNotFound
	}
	if snapshot.Status != snapshots.DraftStatus {
		return ErrSnapshotNotDraft
	}

	lockSession := &locks.LockSession{
		CourseID:  snapshot.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	if err := s.locksService.ValidateLock(ctx, lockSession); err != nil {
		return err
	}

	return nil
}

// CreateBlock calculates position based on AfterBlockID and initiates block creation
func (s *Service) CreateBlock(
	ctx context.Context,
	model *CreateBlock,
	userID, sessionID uuid.UUID,
) (*Block, error) {
	if err := s.validateLock(
		ctx,
		model.SnapshotID,
		userID,
		sessionID,
	); err != nil {
		return nil, err
	}

	leftPos, rightPos, err := s.repo.GetPositionsForMove(
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

	block, err := s.repo.CreateBlock(ctx, model, calculatedPos)
	if err != nil {
		return nil, err
	}

	// Check if rebalance is needed
	if len(calculatedPos) > s.lexoRankThreshold {
		go s.rebalanceSnapshotPositions(context.Background(), model.SnapshotID)
	}

	return block, nil
}

// MoveBlock calculates new block position and updates it in DB
func (s *Service) MoveBlock(
	ctx context.Context,
	blockID, snapshotID, userID, sessionID uuid.UUID,
	afterBlockID *uuid.UUID,
) error {
	if err := s.validateLock(ctx, snapshotID, userID, sessionID); err != nil {
		return err
	}

	if afterBlockID != nil && *afterBlockID == blockID {
		return ErrInvalidBlockForMoveAfter
	}

	leftPos, rightPos, err := s.repo.GetPositionsForMove(
		ctx,
		snapshotID,
		afterBlockID,
	)
	if err != nil {
		return fmt.Errorf("service move block: get positions: %w", err)
	}

	newPosition := CalculateMiddlePosition(leftPos, rightPos)

	err = s.repo.UpdateBlockPosition(ctx, blockID, newPosition)
	if err != nil {
		return fmt.Errorf("service move block: save position: %w", err)
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
	if err := s.validateLock(ctx, snapshotID, userID, sessionID); err != nil {
		return nil, err
	}
	return s.repo.UpdateBlockContent(ctx, blockID, model)
}

func (s *Service) DeleteBlockByID(
	ctx context.Context,
	blockID, snapshotID, userID, sessionID uuid.UUID,
) error {
	if err := s.validateLock(ctx, snapshotID, userID, sessionID); err != nil {
		return err
	}
	return s.repo.DeleteBlockByID(ctx, blockID)
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

// rebalanceSnapshotPositions shrinks overgrown position lines,
// distributing them evenly across the available ASCII range
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
	currentPrev := ""
	for _, block := range blocksList {
		newPos := CalculateMiddlePosition(currentPrev, "")

		if block.Position != newPos {
			err = s.repo.UpdateBlockPosition(ctx, block.ID, newPos)
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
