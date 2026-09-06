package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/content"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

const (
	createContentTaskSQL = `
		INSERT INTO tasks (block_id, snapshot_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at)
		VALUES (:block_id, :snapshot_id, :task_group_id, :name, :patterns, :max_grade, :max_attempts, :available_at, :deadline_at)
		RETURNING block_id
	`

	// Generates the copied blocks' ids explicitly (instead of relying on the
	// column DEFAULT) so a task-type block's `tasks` row can be copied
	// alongside it under the new block_id. Each snapshot generation of a task
	// gets its own tasks row (see the NOTE on the tasks table), so this never
	// collides with the source's still-existing row.
	copyBlocksToSnapshotSQL = `
		WITH source AS (
			SELECT id AS old_id, uuidv7() AS new_id, block_type, data, position
			FROM blocks
			WHERE snapshot_id = $2 AND deleted_at IS NULL
		),
		inserted_blocks AS (
			INSERT INTO blocks (id, snapshot_id, block_type, data, position)
			SELECT new_id, $1, block_type, data, position FROM source
			RETURNING id
		)
		INSERT INTO tasks (block_id, snapshot_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at)
		SELECT s.new_id, $1, t.task_group_id, t.name, t.patterns, t.max_grade, t.max_attempts, t.available_at, t.deadline_at
		FROM source s
		JOIN tasks t ON t.block_id = s.old_id
		WHERE s.block_type = 'task' AND s.new_id IN (SELECT id FROM inserted_blocks)
	`
)

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

		var groupCourseID uuid.UUID
		if err := tx.GetContext(ctx, &groupCourseID, getCourseIDByTaskGroupSQL, task.TaskGroupID); err != nil {
			if err == sql.ErrNoRows {
				return content.ErrTaskGroupNotFound
			}
			return fmt.Errorf("get task group course: %w", err)
		}
		if groupCourseID != command.CourseID {
			return content.ErrTaskGroupNotFound
		}

		patterns := task.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		newTask := tasks.Task{
			ID:          block.ID,
			SnapshotID:  block.SnapshotID,
			TaskGroupID: task.TaskGroupID,
			Name:        task.Name,
			Patterns:    pq.StringArray(patterns),
			MaxGrade:    task.MaxGrade,
			MaxAttempts: task.MaxAttempts,
			AvailableAt: task.AvailableAt,
			DeadlineAt:  task.DeadlineAt,
		}
		taskStmt, err := tx.PrepareNamedContext(ctx, createContentTaskSQL)
		if err != nil {
			return fmt.Errorf("prepare content task: %w", err)
		}
		defer func() {
			if closeErr := taskStmt.Close(); closeErr != nil {
				zap.L().Error("failed to close content task statement", zap.Error(closeErr))
			}
		}()
		if err := taskStmt.GetContext(ctx, &result.TaskID, newTask); err != nil {
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

// PatchBlockContent updates a block's own data and, if it is a task-type
// block, its task fields — all gated by the same editable-snapshot check
// used for every other course-editing mutation.
func (r *PGRepo) PatchBlockContent(ctx context.Context, command content.PatchBlockCommand) (*content.PatchedBlockContent, error) {
	var result content.PatchedBlockContent
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(ctx, tx, command.CourseID, command.SnapshotID, command.Actor.UserID, command.Actor.SessionID); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(ctx, tx, command.BlockID, command.SnapshotID); err != nil {
			return err
		}

		// nil Data must reach the driver as SQL NULL (not an empty string),
		// so that COALESCE leaves the existing column value untouched
		var data any
		if len(command.Data) > 0 {
			data = string(command.Data)
		}
		var block blocks.Block
		if err := tx.QueryRowxContext(ctx, updateBlockContentSQL, command.BlockType, data, command.BlockID).
			StructScan(&block); err != nil {
			return fmt.Errorf("tx update content block: %w", err)
		}
		result.Block = &block

		if command.Task != nil && block.BlockType != "task" {
			return content.ErrInvalidTaskBlock
		}
		if block.BlockType != "task" {
			return nil
		}

		task, err := getOrPatchContentTask(ctx, tx, block.ID, command.Task)
		if err != nil {
			return err
		}
		result.Task = task
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// getOrPatchContentTask applies update to the task attached to blockID (or,
// if update is nil, simply reads its current state) and returns it as
// content.TaskData. A task-type block with no matching tasks row is an
// invariant violation — surfaced as ErrInvalidTaskBlock rather than a raw
// "not found".
func getOrPatchContentTask(
	ctx context.Context,
	tx *sqlx.Tx,
	blockID uuid.UUID,
	update *content.TaskUpdate,
) (*content.TaskData, error) {
	var task tasks.Task

	if update == nil {
		if err := tx.GetContext(ctx, &task, getTaskByIDSQL, blockID); err != nil {
			if err == sql.ErrNoRows {
				return nil, content.ErrInvalidTaskBlock
			}
			return nil, fmt.Errorf("get content task: %w", err)
		}
	} else {
		var patterns any
		if update.Patterns != nil {
			patterns = pq.StringArray(*update.Patterns)
		}
		if err := tx.QueryRowxContext(ctx, updateTaskSQL,
			patterns, update.MaxGrade, update.MaxAttempts, update.AvailableAt, update.DeadlineAt, blockID,
		).StructScan(&task); err != nil {
			if err == sql.ErrNoRows {
				return nil, content.ErrInvalidTaskBlock
			}
			return nil, fmt.Errorf("tx update content task: %w", err)
		}
	}

	return &content.TaskData{
		TaskGroupID: task.TaskGroupID,
		Name:        task.Name,
		Patterns:    []string(task.Patterns),
		MaxGrade:    task.MaxGrade,
		MaxAttempts: task.MaxAttempts,
		AvailableAt: task.AvailableAt,
		DeadlineAt:  task.DeadlineAt,
	}, nil
}

// CopyBlocksToSnapshot copies blocks (and, for task-type blocks, their
// attached task data) from one snapshot to another.
func (r *PGRepo) CopyBlocksToSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	sourceSnapshotID uuid.UUID,
	targetSnapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing block copying query within transaction", zap.String("query", copyBlocksToSnapshotSQL))

	_, err := tx.ExecContext(
		ctx,
		copyBlocksToSnapshotSQL,
		targetSnapshotID,
		sourceSnapshotID,
	)
	if err != nil {
		return fmt.Errorf("tx copy blocks: %w", err)
	}
	return nil
}
