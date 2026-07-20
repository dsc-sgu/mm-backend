package blocks

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type RebalanceWorker struct {
	repo  Repo
	queue chan uuid.UUID

	mu      sync.Mutex
	pending map[uuid.UUID]bool
}

func NewRebalanceWorker(repo Repo, queueSize int) *RebalanceWorker {
	return &RebalanceWorker{
		repo:    repo,
		queue:   make(chan uuid.UUID, queueSize),
		pending: make(map[uuid.UUID]bool),
	}
}

// Enqueue schedules a rebalance for snapshotID, unless one is already
// queued or in progress for it
func (w *RebalanceWorker) Enqueue(snapshotID uuid.UUID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.pending[snapshotID] {
		return
	}

	select {
	case w.queue <- snapshotID:
		w.pending[snapshotID] = true
	default:
		zap.L().Warn(
			"rebalance queue full, dropping trigger",
			zap.String("snapshot_id", snapshotID.String()),
		)
	}
}

// Run processes queued rebalances one at a time until ctx is cancelled.
// It must be started exactly once, as its own goroutine,
// at application startup alongside the other long-running servers.
func (w *RebalanceWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case snapshotID := <-w.queue:
			w.rebalance(ctx, snapshotID)

			w.mu.Lock()
			delete(w.pending, snapshotID)
			w.mu.Unlock()
		}
	}
}

// rebalance shrinks overgrown position lines,
// distributing them evenly across the available alphabet
func (w *RebalanceWorker) rebalance(ctx context.Context, snapshotID uuid.UUID) {
	zap.L().
		Info("Starting position rebalancing for snapshot", zap.String("snapshot_id", snapshotID.String()))

	blocksList, err := w.repo.GetAllBlocksBySnapshotID(ctx, snapshotID)
	if err != nil {
		zap.L().
			Error("rebalance positions: failed to fetch blocks", zap.Error(err))
		return
	}

	// Redistribute positions: block i gets the position at fraction
	// (i+1)/(totalBlocks+1) of the alphabet range, leaving a gap at both
	// ends for future inserts.
	totalBlocks := len(blocksList)
	if totalBlocks == 0 {
		return
	}

	for i, block := range blocksList {
		// Add 1 to the value of totalBlocks to leave a gap at the end of the alphabet
		newPos := indexToPosition(i+1, totalBlocks+1, alphabet)

		if block.Position != newPos {
			err := w.repo.UpdateBlockPosition(ctx, block.ID, newPos)
			if err != nil {
				zap.L().
					Error("rebalance positions: failed to update block position", zap.Error(err))
			}
		}
	}

	zap.L().
		Info("Position rebalancing successfully finished", zap.String("snapshot_id", snapshotID.String()))
}

// indexToPosition converts the integer index
// to 4-char position in 62-char alphabet
func indexToPosition(num, denom int, alphabet string) string {
	var result []byte
	base := len(alphabet)

	for range 4 {
		num *= base
		idx := num / denom
		if idx >= base { // defensive: num < denom*base makes this unreachable
			idx = base - 1
		}

		result = append(result, alphabet[idx])

		num %= denom
		if num == 0 { // exact fraction, no more digits carry information
			break
		}
	}
	return string(result)
}
