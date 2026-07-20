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

	if err := w.repo.RebalanceBlockPositions(ctx, snapshotID); err != nil {
		zap.L().
			Error("rebalance positions failed", zap.Error(err))
		return
	}

	zap.L().
		Info("Position rebalancing successfully finished", zap.String("snapshot_id", snapshotID.String()))
}
