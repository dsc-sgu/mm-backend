package pg

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/content"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

const createContentTaskSQL = `
	INSERT INTO tasks (block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING block_id
`

var _ content.Repo = (*PGRepo)(nil)

func (r *PGRepo) CreateBlockContent(ctx context.Context, command content.CreateBlockCommand) (*content.CreatedBlockContent, error) {
	if command.BlockType == "task" && command.Task == nil {
		return nil, fmt.Errorf("task data is required for task block")
	}

	var result content.CreatedBlockContent
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var snapshot snapshots.Snapshot
		if err := tx.GetContext(ctx, &snapshot, getDraftSnapshotSQL, command.CourseID, command.Actor.UserID); err != nil {
			return fmt.Errorf("get current draft snapshot: %w", err)
		}
		if err := validateEditableSnapshot(ctx, tx, command.CourseID, snapshot.ID, command.Actor.UserID, command.Actor.SessionID); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(ctx, tx, snapshot.ID, command.AfterBlockID)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}
		block := blocks.Block{
			SnapshotID: snapshot.ID,
			BlockType:  command.BlockType,
			Data:       command.Data,
			Position:   blocks.CalculateMiddlePosition(positions.Prev, positions.Next),
		}
		stmt, err := tx.PrepareNamedContext(ctx, createBlockSQL)
		if err != nil {
			return fmt.Errorf("prepare content block: %w", err)
		}
		defer func() {
			if closeErr := stmt.Close(); closeErr != nil {
				zap.L().Error("failed to close content block statement", zap.Error(closeErr))
			}
		}()
		if err := stmt.GetContext(ctx, &block.ID, block); err != nil {
			return fmt.Errorf("create content block: %w", err)
		}
		result.BlockID = block.ID
		result.SnapshotID = block.SnapshotID
		result.PositionLength = len(block.Position)

		if command.BlockType != "task" {
			return nil
		}
		task := command.Task
		patterns := task.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		if err := tx.GetContext(ctx, &result.TaskID, createContentTaskSQL,
			block.ID, task.TaskGroupID, task.Name, pq.Array(patterns),
			task.MaxGrade, task.MaxAttempts, task.AvailableAt, task.DeadlineAt,
		); err != nil {
			return fmt.Errorf("create content task: %w", err)
		}
		zap.L().Debug("created task content", zap.String("block_id", block.ID.String()))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
