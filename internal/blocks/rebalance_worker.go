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

// Enqueue schedules a rebalance for courseID, unless one is already
// queued or in progress for it
func (w *RebalanceWorker) Enqueue(courseID uuid.UUID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.pending[courseID] {
		return
	}

	select {
	case w.queue <- courseID:
		w.pending[courseID] = true
	default:
		zap.L().Warn(
			"rebalance queue full, dropping trigger",
			zap.String("course_id", courseID.String()),
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
		case courseID := <-w.queue:
			w.rebalance(ctx, courseID)

			w.mu.Lock()
			delete(w.pending, courseID)
			w.mu.Unlock()
		}
	}
}

// rebalance shrinks overgrown position lines,
// distributing them evenly across the available alphabet
func (w *RebalanceWorker) rebalance(ctx context.Context, courseID uuid.UUID) {
	zap.L().
		Info("Starting position rebalancing for course", zap.String("course_id", courseID.String()))

	if err := w.repo.RebalanceBlockPositions(ctx, courseID); err != nil {
		zap.L().
			Error("rebalance positions failed", zap.Error(err))
		return
	}

	zap.L().
		Info("Position rebalancing successfully finished", zap.String("course_id", courseID.String()))
}
