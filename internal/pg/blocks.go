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
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

const (
	createBlockSQL = `
		INSERT INTO blocks (snapshot_id, block_type, position)
		VALUES (:snapshot_id, :block_type, :position)
		RETURNING id
	`

	createTextBlockSQL = `
		INSERT INTO text_blocks (block_id, content)
		VALUES ($1, $2)
	`

	createQuizBlockSQL = `
		INSERT INTO quiz_blocks (block_id, content)
		VALUES ($1, $2)
	`

	updateTextBlockSQL = `
		UPDATE text_blocks SET content = $2
		WHERE block_id = $1
		RETURNING content
	`

	updateQuizBlockSQL = `
		UPDATE quiz_blocks SET content = $2
		WHERE block_id = $1
		RETURNING content
	`

	// A block's content lives in the table of its type; a task block has no
	// content row, so its data reads as JSON null.
	getBlockByIDSQL = `
		SELECT b.id, b.snapshot_id, b.block_type,
		       COALESCE(tb.content, qb.content, 'null'::jsonb) AS data,
		       b.position, b.deleted_at
		FROM blocks b
		LEFT JOIN text_blocks tb ON tb.block_id = b.id
		LEFT JOIN quiz_blocks qb ON qb.block_id = b.id
		WHERE b.id = $1 AND b.deleted_at IS NULL
	`

	getBlockSnapshotIDSQL = `
		SELECT snapshot_id
		FROM blocks
		WHERE id = $1 AND deleted_at IS NULL
	`

	getAllBlocksBySnapshotIDSQL = `
		SELECT b.id, b.snapshot_id, b.block_type,
		       COALESCE(tb.content, qb.content, 'null'::jsonb) AS data,
		       b.position
		FROM blocks b
		LEFT JOIN text_blocks tb ON tb.block_id = b.id
		LEFT JOIN quiz_blocks qb ON qb.block_id = b.id
		WHERE b.snapshot_id = $1 AND b.deleted_at IS NULL
		ORDER BY b.position ASC
	`

	updateBlockPositionSQL = `
		UPDATE blocks
		SET position = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	deleteBlockByIDSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	deleteAllBlocksBySnapshotIDSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE snapshot_id = $1 AND deleted_at IS NULL
	`

	deleteAllBlocksByCourseIDSQL = `
		UPDATE blocks
		SET deleted_at = NOW()
		WHERE snapshot_id IN (SELECT id FROM course_snapshots WHERE course_id = $1)
		  AND deleted_at IS NULL
	`

	getBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE id = $1 AND snapshot_id = $2 AND deleted_at IS NULL
	`

	getNextBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE snapshot_id = $1
		  AND position > $2
		  AND deleted_at IS NULL
		ORDER BY position ASC
		LIMIT 1
	`

	getFirstBlockPositionSQL = `
		SELECT position
		FROM blocks
		WHERE snapshot_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC
		LIMIT 1
	`

	lockSnapshotSQL = `
		SELECT course_id, status
		FROM course_snapshots
		WHERE id = $1
		FOR UPDATE
	`

	lockSnapshotForRebalanceSQL = `
		SELECT 1
		FROM course_snapshots
		WHERE id = $1
		FOR UPDATE
	`

	lockCourseLockSQL = `
		SELECT user_id, session_id, (expires_at > NOW()) AS is_valid
		FROM course_locks
		WHERE course_id = $1
		FOR UPDATE
	`

	createTaskSQL = `
		INSERT INTO tasks (task_group_id, name, statement, patterns, max_grade, max_attempts, available_at, deadline_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	createTaskBlockSQL = `
		INSERT INTO task_blocks (block_id, task_id)
		VALUES ($1, $2)
	`

	// Task names are unique among the live tasks of a group in one snapshot.
	taskNameTakenSQL = `
		SELECT EXISTS (
			SELECT 1
			FROM tasks t
			JOIN task_blocks tb ON tb.task_id = t.id
			JOIN blocks b ON b.id = tb.block_id AND b.deleted_at IS NULL
			WHERE b.snapshot_id = $1 AND t.task_group_id = $2 AND t.name = $3
		)
	`

	// Soft-deletes the tasks of snapshotID's deleted blocks that no live
	// block of any snapshot links to anymore: a task created in a draft and
	// dropped before being published. A task a published snapshot links to
	// stays, since attempts may point at it.
	deleteUnlinkedTasksSQL = `
		UPDATE tasks t
		SET deleted_at = NOW()
		WHERE t.deleted_at IS NULL
		  AND t.id IN (
			SELECT tb.task_id
			FROM task_blocks tb
			JOIN blocks b ON b.id = tb.block_id
			WHERE b.snapshot_id = $1
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM task_blocks tb
			JOIN blocks b ON b.id = tb.block_id AND b.deleted_at IS NULL
			WHERE tb.task_id = t.id
		  )
	`

	// Generates the copied blocks' ids explicitly (instead of relying on the
	// column DEFAULT) so a block's content row, or a task-type block's link to
	// its task, can be copied alongside it under the new block_id. The task
	// itself is not copied: the new block links to the same task.
	copyBlocksToSnapshotSQL = `
		WITH source AS (
			SELECT id AS old_id, uuidv7() AS new_id, block_type, position
			FROM blocks
			WHERE snapshot_id = $2 AND deleted_at IS NULL
		),
		inserted_blocks AS (
			INSERT INTO blocks (id, snapshot_id, block_type, position)
			SELECT new_id, $1, block_type, position FROM source
			RETURNING id
		),
		copied_texts AS (
			INSERT INTO text_blocks (block_id, content)
			SELECT s.new_id, tb.content
			FROM source s
			JOIN text_blocks tb ON tb.block_id = s.old_id
			WHERE s.new_id IN (SELECT id FROM inserted_blocks)
		),
		copied_quizzes AS (
			INSERT INTO quiz_blocks (block_id, content)
			SELECT s.new_id, qb.content
			FROM source s
			JOIN quiz_blocks qb ON qb.block_id = s.old_id
			WHERE s.new_id IN (SELECT id FROM inserted_blocks)
		)
		INSERT INTO task_blocks (block_id, task_id)
		SELECT s.new_id, tb.task_id
		FROM source s
		JOIN task_blocks tb ON tb.block_id = s.old_id
		WHERE s.new_id IN (SELECT id FROM inserted_blocks)
	`
)

// validateEditableSnapshot locks the snapshot row and the course's lock row
// within tx and verifies the snapshot belongs to courseID, is a draft, and
// is currently locked by user/session
func validateEditableSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID, snapshotID uuid.UUID,
	userID, sessionID uuid.UUID,
) error {
	var snapshot snapshots.Snapshot
	err := tx.GetContext(ctx, &snapshot, lockSnapshotSQL, snapshotID)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrSnapshotNotFound
		}
		return fmt.Errorf("lock snapshot: %w", err)
	}
	if snapshot.CourseID != courseID {
		return blocks.ErrSnapshotNotFound
	}
	if snapshot.Status != snapshots.DraftStatus {
		return blocks.ErrSnapshotNotDraft
	}

	var lock locks.Lock
	err = tx.GetContext(ctx, &lock, lockCourseLockSQL, snapshot.CourseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return locks.ErrLockNotFound
		}
		return fmt.Errorf("lock course lock: %w", err)
	}
	if lock.UserID != userID || lock.SessionID != sessionID {
		return locks.ErrLockHeldByAnother
	}
	if !lock.IsValid {
		return locks.ErrLockExpired
	}

	return nil
}

// validateViewableSnapshot verifies snapshotID belongs to courseID and that
// userID may view it, and, if the snapshot is a draft, that userID/sessionID
// currently hold its course's edit lock
func (r *PGRepo) validateViewableSnapshot(
	ctx context.Context,
	courseID, snapshotID, userID, sessionID uuid.UUID,
) (*snapshots.Snapshot, error) {
	snapshot, err := r.GetSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	if snapshot == nil || snapshot.CourseID != courseID {
		return nil, blocks.ErrSnapshotNotFound
	}

	member, err := r.GetMember(ctx, userID, courseID)
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	if member == nil || !member.IsActive ||
		member.Role != membership.TeacherRole {
		return nil, blocks.ErrPermissionDenied
	}

	if snapshot.Status == snapshots.DraftStatus {
		lock, err := r.GetLock(ctx, courseID)
		if err != nil {
			return nil, fmt.Errorf("get lock: %w", err)
		}
		if lock == nil {
			return nil, locks.ErrLockNotFound
		}
		if lock.UserID != userID || lock.SessionID != sessionID {
			return nil, locks.ErrLockHeldByAnother
		}
		if !lock.IsValid {
			return nil, locks.ErrLockExpired
		}
	}

	return snapshot, nil
}

// blockBelongsToSnapshot verifies blockID is a valid block
// of the snapshot with snapshotID within tx
func blockBelongsToSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	blockID, snapshotID uuid.UUID,
) error {
	var actualSnapshotID uuid.UUID
	err := tx.GetContext(ctx, &actualSnapshotID, getBlockSnapshotIDSQL, blockID)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrBlockNotFound
		}
		return fmt.Errorf("get block: %w", err)
	}
	if actualSnapshotID != snapshotID {
		return blocks.ErrBlockNotFound
	}
	return nil
}

// blockContentSQL returns the insert or update query for the content table
// of blockType. A task block has no content table, so data is not allowed
// for it.
func blockContentSQL(blockType string, update bool) (string, error) {
	switch blockType {
	case "text":
		if update {
			return updateTextBlockSQL, nil
		}
		return createTextBlockSQL, nil
	case "quiz":
		if update {
			return updateQuizBlockSQL, nil
		}
		return createQuizBlockSQL, nil
	}
	return "", blocks.ErrInvalidBlockData
}

// getPositionsForMoveTx returns prev and next block positions to calculate a
// new position between them
func getPositionsForMoveTx(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) (blocks.AdjacentPositions, error) {
	// If after_block_id is null, get first block position
	if afterBlockID == nil {
		var rightPos string
		err := tx.GetContext(
			ctx,
			&rightPos,
			getFirstBlockPositionSQL,
			snapshotID,
		)
		if err != nil && err != sql.ErrNoRows {
			return blocks.AdjacentPositions{}, fmt.Errorf(
				"get first block position: %w",
				err,
			)
		}
		return blocks.AdjacentPositions{Next: rightPos}, nil
	}

	// Else get prev and next block positions
	var leftPos string
	err := tx.GetContext(
		ctx,
		&leftPos,
		getBlockPositionSQL,
		*afterBlockID,
		snapshotID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return blocks.AdjacentPositions{}, fmt.Errorf(
				"%w: %s",
				blocks.ErrAfterBlockNotFound,
				*afterBlockID,
			)
		}
		return blocks.AdjacentPositions{}, fmt.Errorf(
			"get prev block position: %w",
			err,
		)
	}

	var rightPos string
	err = tx.GetContext(
		ctx,
		&rightPos,
		getNextBlockPositionSQL,
		snapshotID,
		leftPos,
	)
	if err != nil && err != sql.ErrNoRows {
		return blocks.AdjacentPositions{}, fmt.Errorf(
			"get next block position: %w",
			err,
		)
	}

	return blocks.AdjacentPositions{Prev: leftPos, Next: rightPos}, nil
}

func (r *PGRepo) CreateBlock(
	ctx context.Context,
	command blocks.CreateBlockCommand,
) (*blocks.CreatedBlock, error) {
	if command.BlockType == "task" && command.Task == nil {
		return nil, fmt.Errorf("task data is required for task block")
	}
	var contentSQL string
	if command.BlockType != "task" {
		var err error
		if contentSQL, err = blockContentSQL(command.BlockType, false); err != nil {
			return nil, err
		}
		if len(command.Data) == 0 {
			return nil, blocks.ErrInvalidBlockData
		}
	} else if len(command.Data) > 0 {
		return nil, blocks.ErrInvalidBlockData
	}

	var result blocks.CreatedBlock
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var snapshot snapshots.Snapshot
		if err := tx.GetContext(
			ctx,
			&snapshot,
			getDraftSnapshotSQL,
			command.CourseID,
			command.Actor.UserID,
		); err != nil {
			return fmt.Errorf("get current draft snapshot: %w", err)
		}
		if err := validateEditableSnapshot(
			ctx,
			tx,
			command.CourseID,
			snapshot.ID,
			command.Actor.UserID,
			command.Actor.SessionID,
		); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			snapshot.ID,
			command.AfterBlockID,
		)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}
		block := blocks.Block{
			SnapshotID: snapshot.ID,
			BlockType:  command.BlockType,
			Position: blocks.CalculateMiddlePosition(
				positions.Prev,
				positions.Next,
			),
		}
		stmt, err := tx.PrepareNamedContext(ctx, createBlockSQL)
		if err != nil {
			return fmt.Errorf("prepare block: %w", err)
		}
		defer func() {
			if closeErr := stmt.Close(); closeErr != nil {
				zap.L().
					Error("failed to close block statement", zap.Error(closeErr))
			}
		}()
		if err := stmt.GetContext(ctx, &block.ID, block); err != nil {
			return fmt.Errorf("create block: %w", err)
		}
		result.BlockID = block.ID
		result.SnapshotID = block.SnapshotID
		result.PositionLength = len(block.Position)

		if command.BlockType != "task" {
			if _, err := tx.ExecContext(
				ctx,
				contentSQL,
				block.ID,
				string(command.Data),
			); err != nil {
				return fmt.Errorf("create block content: %w", err)
			}
			return nil
		}
		task := command.Task

		var groupCourseID uuid.UUID
		if err := tx.GetContext(
			ctx,
			&groupCourseID,
			getCourseIDByTaskGroupSQL,
			task.TaskGroupID,
		); err != nil {
			if err == sql.ErrNoRows {
				return blocks.ErrTaskGroupNotFound
			}
			return fmt.Errorf("get task group course: %w", err)
		}
		if groupCourseID != command.CourseID {
			return blocks.ErrTaskGroupNotFound
		}

		var taken bool
		if err := tx.GetContext(
			ctx,
			&taken,
			taskNameTakenSQL,
			snapshot.ID,
			task.TaskGroupID,
			task.Name,
		); err != nil {
			return fmt.Errorf("check task name: %w", err)
		}
		if taken {
			return blocks.ErrTaskNameTaken
		}

		patterns := task.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		// nil Statement must reach the driver as SQL NULL, and a non-empty
		// one as a string: lib/pq sends []byte as bytea, which jsonb rejects
		var statement any
		if len(task.Statement) > 0 {
			statement = string(task.Statement)
		}
		var taskID uuid.UUID
		if err := tx.GetContext(
			ctx,
			&taskID,
			createTaskSQL,
			task.TaskGroupID,
			task.Name,
			statement,
			pq.StringArray(patterns),
			task.MaxGrade,
			task.MaxAttempts,
			task.AvailableAt,
			task.DeadlineAt,
		); err != nil {
			return fmt.Errorf("create task: %w", err)
		}
		if _, err := tx.ExecContext(ctx, createTaskBlockSQL, block.ID, taskID); err != nil {
			return fmt.Errorf("link task block: %w", err)
		}
		result.TaskID = &taskID
		zap.L().
			Debug("created task block", zap.String("block_id", block.ID.String()))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PatchBlock updates a block's own data, gated by the same editable-snapshot
// check used for every other course-editing mutation.
func (r *PGRepo) PatchBlock(
	ctx context.Context,
	command blocks.PatchBlockCommand,
) (*blocks.PatchedBlock, error) {
	var result blocks.PatchedBlock
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			command.CourseID,
			command.SnapshotID,
			command.Actor.UserID,
			command.Actor.SessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(
			ctx,
			tx,
			command.BlockID,
			command.SnapshotID,
		); err != nil {
			return err
		}

		var block blocks.Block
		if err := tx.GetContext(ctx, &block, getBlockByIDSQL, command.BlockID); err != nil {
			return fmt.Errorf("tx get block: %w", err)
		}
		if command.BlockType != nil && *command.BlockType != block.BlockType {
			return blocks.ErrBlockTypeChanged
		}
		if len(command.Data) > 0 {
			contentSQL, err := blockContentSQL(block.BlockType, true)
			if err != nil {
				return err
			}
			if err := tx.GetContext(
				ctx,
				&block.Data,
				contentSQL,
				block.ID,
				string(command.Data),
			); err != nil {
				return fmt.Errorf("tx update block content: %w", err)
			}
		}
		result.Block = &block
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *PGRepo) GetBlockByID(
	ctx context.Context,
	editCtx blocks.EditContext,
	ref blocks.BlockRef,
) (*blocks.Block, error) {
	if _, err := r.validateViewableSnapshot(
		ctx,
		ref.CourseID,
		ref.SnapshotID,
		editCtx.UserID,
		editCtx.SessionID,
	); err != nil {
		return nil, err
	}

	zap.L().Debug("Executing query", zap.String("query", getBlockByIDSQL))

	var block blocks.Block
	err := r.db.GetContext(ctx, &block, getBlockByIDSQL, ref.BlockID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, blocks.ErrBlockNotFound
		}
		return nil, err
	}
	if block.SnapshotID != ref.SnapshotID {
		return nil, blocks.ErrBlockNotFound
	}
	return &block, nil
}

func (r *PGRepo) GetAllBlocksBySnapshotID(
	ctx context.Context,
	snapshotID uuid.UUID,
) ([]*blocks.Block, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getAllBlocksBySnapshotIDSQL))

	var blockList []*blocks.Block
	rows, err := r.db.QueryxContext(
		ctx,
		getAllBlocksBySnapshotIDSQL,
		snapshotID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	for rows.Next() {
		var block blocks.Block
		if err := rows.StructScan(&block); err != nil {
			return nil, err
		}
		blockList = append(blockList, &block)
	}
	if err = rows.Err(); err != nil {
		return blockList, err
	}
	return blockList, nil
}

func (r *PGRepo) MoveBlock(
	ctx context.Context,
	editCtx blocks.EditContext,
	ref blocks.BlockRef,
	afterBlockID *uuid.UUID,
) (string, error) {
	var newPosition string

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			ref.CourseID,
			ref.SnapshotID,
			editCtx.UserID,
			editCtx.SessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(
			ctx,
			tx,
			ref.BlockID,
			ref.SnapshotID,
		); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			ref.SnapshotID,
			afterBlockID,
		)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}
		newPosition = blocks.CalculateMiddlePosition(
			positions.Prev,
			positions.Next,
		)

		zap.L().
			Debug("Executing query within transaction", zap.String("query", updateBlockPositionSQL))

		res, err := tx.ExecContext(
			ctx,
			updateBlockPositionSQL,
			newPosition,
			ref.BlockID,
		)
		if err != nil {
			return fmt.Errorf("tx update block position: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("tx update block position rows affected: %w", err)
		}
		if affected == 0 {
			return blocks.ErrBlockNotFound
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return newPosition, nil
}

// RebalanceBlockPositions recomputes and updates every block's position
// for a specific snapshot to distribute the blocks evenly across the alphabet
func (r *PGRepo) RebalanceBlockPositions(
	ctx context.Context,
	snapshotID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var dummy int
		err := tx.GetContext(
			ctx,
			&dummy,
			lockSnapshotForRebalanceSQL,
			snapshotID,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("lock snapshot: %w", err)
		}

		var blocksList []*blocks.Block
		if err := tx.SelectContext(
			ctx,
			&blocksList,
			getAllBlocksBySnapshotIDSQL,
			snapshotID,
		); err != nil {
			return fmt.Errorf("get blocks: %w", err)
		}

		// Redistribute positions: block i gets the position at fraction
		// (i+1)/(totalBlocks+1) of the alphabet range, leaving a gap at both
		// ends for future inserts.
		totalBlocks := len(blocksList)
		for i, block := range blocksList {
			// Add 1 to the value of totalBlocks to leave a gap at the end of the alphabet
			newPos := blocks.IndexToPosition(i+1, totalBlocks+1)
			if block.Position == newPos {
				continue
			}
			if _, err := tx.ExecContext(
				ctx,
				updateBlockPositionSQL,
				newPos,
				block.ID,
			); err != nil {
				return fmt.Errorf("update block position: %w", err)
			}
		}

		return nil
	})
}

func (r *PGRepo) DeleteBlockByID(
	ctx context.Context,
	editCtx blocks.EditContext,
	ref blocks.BlockRef,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := validateEditableSnapshot(
			ctx,
			tx,
			ref.CourseID,
			ref.SnapshotID,
			editCtx.UserID,
			editCtx.SessionID,
		); err != nil {
			return err
		}
		if err := blockBelongsToSnapshot(
			ctx,
			tx,
			ref.BlockID,
			ref.SnapshotID,
		); err != nil {
			return err
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", deleteBlockByIDSQL))

		_, err := tx.ExecContext(ctx, deleteBlockByIDSQL, ref.BlockID)
		if err != nil {
			return fmt.Errorf("tx delete block: %w", err)
		}

		return deleteUnlinkedTasks(ctx, tx, ref.SnapshotID)
	})
}

func (r *PGRepo) DeleteAllBlocksBySnapshotID(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing blocks delete within transaction", zap.String("query", deleteAllBlocksBySnapshotIDSQL))

	if snapshotID == uuid.Nil {
		return fmt.Errorf("blocks delete: snapshot id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteAllBlocksBySnapshotIDSQL, snapshotID)
	if err != nil {
		return fmt.Errorf("tx soft delete blocks by snapshot: %w", err)
	}
	return deleteUnlinkedTasks(ctx, tx, snapshotID)
}

// deleteUnlinkedTasks soft-deletes the tasks that lost their last live block
// when blocks of snapshotID were deleted.
func deleteUnlinkedTasks(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshotID uuid.UUID,
) error {
	zap.L().
		Debug("Executing query within transaction", zap.String("query", deleteUnlinkedTasksSQL))

	if _, err := tx.ExecContext(ctx, deleteUnlinkedTasksSQL, snapshotID); err != nil {
		return fmt.Errorf("tx delete unlinked tasks: %w", err)
	}
	return nil
}

func (r *PGRepo) DeleteAllBlocksByCourseID(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
) error {
	zap.L().
		Debug("Executing blocks delete within transaction", zap.String("query", deleteAllBlocksByCourseIDSQL))

	if courseID == uuid.Nil {
		return fmt.Errorf("blocks delete: course id is nil")
	}

	_, err := tx.ExecContext(ctx, deleteAllBlocksByCourseIDSQL, courseID)
	if err != nil {
		return fmt.Errorf("tx soft delete blocks by course: %w", err)
	}
	return nil
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
