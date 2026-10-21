package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

const blockColumns = `id, version, course_id, block_type, data, position, draft, deleted, created_by, base_version`

// rawBlockColumns reads a block version as stored, for copying and merging.
// A task block's NULL data comes back as the JSON null, which Go cannot
// scan a NULL into; jsonbArg writes it back as NULL.
const rawBlockColumns = `id, version, course_id, block_type, COALESCE(data, 'null'::jsonb) AS data, position, draft, deleted, created_by, base_version`

// blockView selects, for every block of the course named by courseParam,
// the version the viewer named by viewerParam sees: their own draft if they
// have one, else the latest published version. With viewerParam = "NULL" it
// is the published course. Deleted blocks are included; callers filter them.
func blockView(courseParam, viewerParam string) string {
	return `
		SELECT DISTINCT ON (id) ` + blockColumns + `
		FROM blocks
		WHERE course_id = ` + courseParam + `
		  AND (NOT draft OR created_by = ` + viewerParam + `)
		ORDER BY id, draft DESC, version DESC
	`
}

// A task block has no data of its own; it reads as a reference to its task
// built from task_blocks.
const viewColumnsWithTaskData = `
	v.id, v.version, v.course_id, v.block_type,
	COALESCE(v.data, jsonb_build_object('version', 1, 'taskId', tb.task_id)) AS data,
	v.position, v.draft, v.deleted, v.created_by, v.base_version
`

var (
	// The blocks of a deleted course are gone with it.
	getCourseBlocksSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT ` + viewColumnsWithTaskData + `
		FROM v
		JOIN courses c ON c.id = v.course_id AND c.deleted_at IS NULL
		LEFT JOIN task_blocks tb ON tb.block_id = v.id
		WHERE NOT v.deleted
		ORDER BY v.position ASC, v.id ASC
	`

	getBlockSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT ` + viewColumnsWithTaskData + `
		FROM v
		JOIN courses c ON c.id = v.course_id AND c.deleted_at IS NULL
		LEFT JOIN task_blocks tb ON tb.block_id = v.id
		WHERE v.id = $3 AND NOT v.deleted
	`

	getFirstPositionSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT position FROM v
		WHERE NOT deleted
		ORDER BY position ASC, id ASC
		LIMIT 1
	`

	getPositionSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT position FROM v
		WHERE id = $3 AND NOT deleted
	`

	getNextPositionSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT position FROM v
		WHERE NOT deleted AND (position, id) > ($3, $4)
		ORDER BY position ASC, id ASC
		LIMIT 1
	`

	// Task names are unique among the live tasks of a group as the viewer
	// sees the course.
	taskNameTakenSQL = `
		WITH v AS (` + blockView("$1", "$2") + `)
		SELECT EXISTS (
			SELECT 1
			FROM tasks t
			JOIN task_blocks tb ON tb.task_id = t.id
			JOIN v ON v.id = tb.block_id AND NOT v.deleted
			WHERE t.task_group_id = $3 AND t.name = $4 AND t.deleted_at IS NULL
		)
	`
)

const (
	// A course row is locked for share by every block edit and for update
	// by a publish, a restore and a rebalance, so those see no edit in
	// flight. It also answers whether the course exists.
	lockCourseForShareSQL = `
		SELECT 1 FROM courses WHERE id = $1 AND deleted_at IS NULL FOR SHARE
	`

	lockCourseForUpdateSQL = `
		SELECT 1 FROM courses WHERE id = $1 AND deleted_at IS NULL FOR UPDATE
	`

	// Serializes the creation of versions of one block, so two drafts of it
	// started at once do not pick the same version number.
	lockBlockSQL = `
		SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))
	`

	getOwnDraftSQL = `
		SELECT ` + rawBlockColumns + `
		FROM blocks
		WHERE id = $1 AND draft AND created_by = $2
	`

	// The latest published version of a block.
	getHeadSQL = `
		SELECT ` + rawBlockColumns + `
		FROM blocks
		WHERE id = $1 AND NOT draft
		ORDER BY version DESC
		LIMIT 1
	`

	getBlockVersionSQL = `
		SELECT ` + rawBlockColumns + `
		FROM blocks
		WHERE id = $1 AND version = $2
	`

	nextVersionSQL = `
		SELECT COALESCE(MAX(version), 0) + 1 FROM blocks WHERE id = $1
	`

	insertBlockSQL = `
		INSERT INTO blocks (id, version, course_id, block_type, data, position, draft, deleted, created_by, base_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	insertNewBlockSQL = `
		INSERT INTO blocks (version, course_id, block_type, data, position, draft, deleted, created_by, base_version)
		VALUES (1, $1, $2, $3, $4, true, false, $5, NULL)
		RETURNING id
	`

	updateDraftDataSQL = `
		UPDATE blocks SET data = $3
		WHERE id = $1 AND version = $2
	`

	updateDraftPositionSQL = `
		UPDATE blocks SET position = $3
		WHERE id = $1 AND version = $2
	`

	updateDraftDeletedSQL = `
		UPDATE blocks SET deleted = $3
		WHERE id = $1 AND version = $2
	`

	deleteBlockVersionSQL = `
		DELETE FROM blocks WHERE id = $1 AND version = $2
	`

	getDistinctPositionsSQL = `
		SELECT DISTINCT position FROM blocks
		WHERE course_id = $1
		ORDER BY position ASC
	`

	// One statement, so every row is matched against the positions as they
	// were before it ran and no row is moved twice.
	remapPositionsSQL = `
		UPDATE blocks b
		SET position = m.new_position
		FROM unnest($2::text[], $3::text[]) AS m(old_position, new_position)
		WHERE b.course_id = $1 AND b.position = m.old_position COLLATE "C"
	`

	createTaskSQL = `
		INSERT INTO tasks (task_group_id, name, statement, patterns, max_grade, max_attempts, available_at, deadline_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	createTaskGroupSQL = `
		INSERT INTO task_groups (course_id, name)
		VALUES ($1, $2)
		RETURNING id
	`

	createTaskBlockSQL = `
		INSERT INTO task_blocks (block_id, task_id)
		VALUES ($1, $2)
	`

	// Soft-deletes the tasks of the given blocks that no live block links
	// to anymore: a block is live while its latest published version is
	// not deleted or any draft of it is not deleted. A task group left
	// without live tasks is deleted with its last task. The final UPDATE
	// does not see the CTE's changes, so the tasks deleted by it are
	// excluded by id.
	deleteUnlinkedTasksSQL = `
		WITH deleted_tasks AS (
			UPDATE tasks t
			SET deleted_at = NOW()
			WHERE t.deleted_at IS NULL
			  AND t.id IN (
				SELECT tb.task_id FROM task_blocks tb WHERE tb.block_id = ANY($1::uuid[])
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM task_blocks tb
				WHERE tb.task_id = t.id
				  AND (
					EXISTS (
						SELECT 1 FROM blocks b
						WHERE b.id = tb.block_id AND b.draft AND NOT b.deleted
					)
					OR NOT COALESCE((
						SELECT b.deleted FROM blocks b
						WHERE b.id = tb.block_id AND NOT b.draft
						ORDER BY b.version DESC
						LIMIT 1
					), true)
				  )
			  )
			RETURNING t.id, t.task_group_id
		)
		UPDATE task_groups g
		SET deleted_at = NOW()
		WHERE g.deleted_at IS NULL
		  AND g.id IN (SELECT task_group_id FROM deleted_tasks)
		  AND NOT EXISTS (
			SELECT 1
			FROM tasks t
			WHERE t.task_group_id = g.id
			  AND t.deleted_at IS NULL
			  AND t.id NOT IN (SELECT id FROM deleted_tasks)
		  )
	`

	// Drops the task links of blocks that have no version left (drafts
	// that were discarded before being published).
	deleteOrphanTaskBlocksSQL = `
		DELETE FROM task_blocks tb
		WHERE tb.block_id = ANY($1::uuid[])
		  AND NOT EXISTS (SELECT 1 FROM blocks b WHERE b.id = tb.block_id)
	`
)

// jsonbArg passes a JSON document to the driver as SQL NULL when empty or
// the JSON null, and as a string otherwise: lib/pq sends []byte as bytea,
// which jsonb rejects.
func jsonbArg(data json.RawMessage) any {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return string(data)
}

// lockCourse takes the course row for share within tx, failing with
// ErrCourseNotFound if there is no live course.
func lockCourse(ctx context.Context, tx *sqlx.Tx, courseID uuid.UUID) error {
	var one int
	if err := tx.GetContext(ctx, &one, lockCourseForShareSQL, courseID); err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrCourseNotFound
		}
		return fmt.Errorf("lock course: %w", err)
	}
	return nil
}

// lockCourseForUpdate takes the course row for update within tx, failing
// with ErrCourseNotFound if there is no live course.
func lockCourseForUpdate(ctx context.Context, tx *sqlx.Tx, courseID uuid.UUID) error {
	var one int
	if err := tx.GetContext(ctx, &one, lockCourseForUpdateSQL, courseID); err != nil {
		if err == sql.ErrNoRows {
			return blocks.ErrCourseNotFound
		}
		return fmt.Errorf("lock course: %w", err)
	}
	return nil
}

// insertBlock inserts one version of a block within tx.
func insertBlock(ctx context.Context, tx *sqlx.Tx, b *blocks.Block) error {
	_, err := tx.ExecContext(
		ctx,
		insertBlockSQL,
		b.ID,
		b.Version,
		b.CourseID,
		b.BlockType,
		jsonbArg(b.Data),
		b.Position,
		b.Draft,
		b.Deleted,
		b.CreatedBy,
		b.BaseVersion,
	)
	if err != nil {
		return fmt.Errorf("insert block version: %w", err)
	}
	return nil
}

// nextVersion returns the version number for a new version of a block.
func nextVersion(ctx context.Context, tx *sqlx.Tx, blockID uuid.UUID) (int, error) {
	var version int
	if err := tx.GetContext(ctx, &version, nextVersionSQL, blockID); err != nil {
		return 0, fmt.Errorf("next block version: %w", err)
	}
	return version, nil
}

// getHead returns the latest published version of a block within tx, or
// nil if the block was never published.
func getHead(ctx context.Context, tx *sqlx.Tx, blockID uuid.UUID) (*blocks.Block, error) {
	var head blocks.Block
	if err := tx.GetContext(ctx, &head, getHeadSQL, blockID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get block head: %w", err)
	}
	return &head, nil
}

// ensureDraft returns the user's draft of a live block of the course. The
// user edits from seenVersion, the version they loaded: their own draft,
// which is returned as it is, or a published version, which is copied
// into a new draft with that version as its base, whether or not a later
// one was published since. A draft at another version than the one seen
// means the user edits the block from two places and is a conflict.
func ensureDraft(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID, blockID, userID uuid.UUID,
	seenVersion int,
) (*blocks.Block, error) {
	if _, err := tx.ExecContext(ctx, lockBlockSQL, blockID); err != nil {
		return nil, fmt.Errorf("lock block: %w", err)
	}

	var draft blocks.Block
	err := tx.GetContext(ctx, &draft, getOwnDraftSQL, blockID, userID)
	if err == nil {
		if draft.CourseID != courseID || draft.Deleted {
			return nil, blocks.ErrBlockNotFound
		}
		if draft.Version != seenVersion {
			return nil, blocks.ErrDraftVersionConflict
		}
		return &draft, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get own draft: %w", err)
	}

	var base blocks.Block
	err = tx.GetContext(ctx, &base, getBlockVersionSQL, blockID, seenVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, blocks.ErrBlockVersionNotFound
		}
		return nil, fmt.Errorf("get seen version: %w", err)
	}
	if base.CourseID != courseID || base.Draft || base.Deleted {
		return nil, blocks.ErrBlockVersionNotFound
	}

	version, err := nextVersion(ctx, tx, blockID)
	if err != nil {
		return nil, err
	}
	draft = base
	draft.Version = version
	draft.Draft = true
	draft.CreatedBy = userID
	draft.BaseVersion = &base.Version
	if err := insertBlock(ctx, tx, &draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

// getPositionsForMoveTx returns the positions of the blocks around the
// insertion point, as the viewer sees the course.
func getPositionsForMoveTx(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID, viewerID uuid.UUID,
	afterBlockID *uuid.UUID,
) (blocks.AdjacentPositions, error) {
	// If after_block_id is null, get first block position
	if afterBlockID == nil {
		var rightPos string
		err := tx.GetContext(ctx, &rightPos, getFirstPositionSQL, courseID, viewerID)
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
	err := tx.GetContext(ctx, &leftPos, getPositionSQL, courseID, viewerID, *afterBlockID)
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
		getNextPositionSQL,
		courseID,
		viewerID,
		leftPos,
		*afterBlockID,
	)
	if err != nil && err != sql.ErrNoRows {
		return blocks.AdjacentPositions{}, fmt.Errorf(
			"get next block position: %w",
			err,
		)
	}

	return blocks.AdjacentPositions{Prev: leftPos, Next: rightPos}, nil
}

// CreateBlock creates a block as a draft of the actor and, for a task
// block, its task.
func (r *PGRepo) CreateBlock(
	ctx context.Context,
	command blocks.CreateBlockCommand,
) (*blocks.CreatedBlock, error) {
	userID := command.Actor.UserID

	var result blocks.CreatedBlock
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourse(ctx, tx, command.CourseID); err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			command.CourseID,
			userID,
			command.AfterBlockID,
		)
		if err != nil {
			return fmt.Errorf("resolve positions: %w", err)
		}
		position := blocks.CalculateMiddlePosition(
			positions.Prev,
			positions.Next,
		)
		var blockID uuid.UUID
		if err := tx.GetContext(
			ctx,
			&blockID,
			insertNewBlockSQL,
			command.CourseID,
			command.BlockType,
			jsonbArg(command.Data),
			position,
			userID,
		); err != nil {
			return fmt.Errorf("create block: %w", err)
		}
		result.BlockID = blockID
		result.PositionLength = len(position)

		if command.Task == nil {
			return nil
		}
		task := command.Task

		groupID, err := resolveTaskGroup(ctx, tx, command.CourseID, task.TaskGroup)
		if err != nil {
			return err
		}
		result.TaskGroupID = &groupID

		var taken bool
		if err := tx.GetContext(
			ctx,
			&taken,
			taskNameTakenSQL,
			command.CourseID,
			userID,
			groupID,
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
		var taskID uuid.UUID
		if err := tx.GetContext(
			ctx,
			&taskID,
			createTaskSQL,
			groupID,
			task.Name,
			jsonbArg(task.Statement),
			pq.StringArray(patterns),
			task.MaxGrade,
			task.MaxAttempts,
			task.AvailableAt,
			task.DeadlineAt,
		); err != nil {
			return fmt.Errorf("create task: %w", err)
		}
		if _, err := tx.ExecContext(ctx, createTaskBlockSQL, blockID, taskID); err != nil {
			return fmt.Errorf("link task block: %w", err)
		}
		result.TaskID = &taskID
		zap.L().
			Debug("created task block", zap.String("block_id", blockID.String()))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// resolveTaskGroup returns the id of the group a new task goes to: the
// existing group ref.ID of courseID, or a group created from ref.New.
func resolveTaskGroup(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
	ref blocks.TaskGroupRef,
) (uuid.UUID, error) {
	if ref.New != nil {
		var groupID uuid.UUID
		err := tx.GetContext(ctx, &groupID, createTaskGroupSQL, courseID, ref.New.Name)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return uuid.Nil, blocks.ErrTaskGroupNameTaken
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("create task group: %w", err)
		}
		return groupID, nil
	}

	var groupCourseID uuid.UUID
	if err := tx.GetContext(
		ctx,
		&groupCourseID,
		getCourseIDByTaskGroupSQL,
		*ref.ID,
	); err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, blocks.ErrTaskGroupNotFound
		}
		return uuid.Nil, fmt.Errorf("get task group course: %w", err)
	}
	if groupCourseID != courseID {
		return uuid.Nil, blocks.ErrTaskGroupNotFound
	}
	return *ref.ID, nil
}

// PatchBlock updates a block's own data in the actor's draft of it.
func (r *PGRepo) PatchBlock(
	ctx context.Context,
	command blocks.PatchBlockCommand,
) (*blocks.PatchedBlock, error) {
	userID := command.Actor.UserID

	var result blocks.PatchedBlock
	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourse(ctx, tx, command.CourseID); err != nil {
			return err
		}
		draft, err := ensureDraft(ctx, tx, command.CourseID, command.BlockID, userID, command.Version)
		if err != nil {
			return err
		}
		if command.BlockType != nil && *command.BlockType != draft.BlockType {
			return blocks.ErrBlockTypeChanged
		}
		if len(command.Data) > 0 {
			if draft.BlockType == "task" {
				return blocks.ErrInvalidBlockData
			}
			if _, err := tx.ExecContext(
				ctx,
				updateDraftDataSQL,
				draft.ID,
				draft.Version,
				string(command.Data),
			); err != nil {
				return fmt.Errorf("tx update block data: %w", err)
			}
		}

		var block blocks.Block
		if err := tx.GetContext(
			ctx,
			&block,
			getBlockSQL,
			command.CourseID,
			userID,
			command.BlockID,
		); err != nil {
			return fmt.Errorf("tx get block: %w", err)
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
	viewerID uuid.UUID,
	ref blocks.BlockRef,
) (*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getBlockSQL))

	var block blocks.Block
	err := r.db.GetContext(ctx, &block, getBlockSQL, ref.CourseID, viewerID, ref.BlockID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, blocks.ErrBlockNotFound
		}
		return nil, err
	}
	return &block, nil
}

func (r *PGRepo) GetCourseBlocks(
	ctx context.Context,
	courseID, viewerID uuid.UUID,
) ([]*blocks.Block, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseBlocksSQL))

	blockList := make([]*blocks.Block, 0)
	if err := r.db.SelectContext(
		ctx,
		&blockList,
		getCourseBlocksSQL,
		courseID,
		viewerID,
	); err != nil {
		return nil, fmt.Errorf("get course blocks: %w", err)
	}
	return blockList, nil
}

// MoveBlock moves the block in the actor's draft so that it comes after
// afterBlockID, or first when afterBlockID is nil.
func (r *PGRepo) MoveBlock(
	ctx context.Context,
	editCtx blocks.EditContext,
	ref blocks.BlockRef,
	afterBlockID *uuid.UUID,
) (string, error) {
	var newPosition string

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourse(ctx, tx, ref.CourseID); err != nil {
			return err
		}
		draft, err := ensureDraft(ctx, tx, ref.CourseID, ref.BlockID, editCtx.UserID, ref.Version)
		if err != nil {
			return err
		}

		positions, err := getPositionsForMoveTx(
			ctx,
			tx,
			ref.CourseID,
			editCtx.UserID,
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
			Debug("Executing query within transaction", zap.String("query", updateDraftPositionSQL))

		if _, err := tx.ExecContext(
			ctx,
			updateDraftPositionSQL,
			draft.ID,
			draft.Version,
			newPosition,
		); err != nil {
			return fmt.Errorf("tx update block position: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return newPosition, nil
}

// RebalanceBlockPositions shortens the positions of a course's blocks by
// mapping every distinct position among all their versions to one evenly
// spread across the alphabet. The mapping keeps the order of positions and
// which of them are equal, so published versions still compare as they did.
func (r *PGRepo) RebalanceBlockPositions(
	ctx context.Context,
	courseID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourseForUpdate(ctx, tx, courseID); err != nil {
			if errors.Is(err, blocks.ErrCourseNotFound) {
				return nil
			}
			return err
		}

		var oldPositions []string
		if err := tx.SelectContext(
			ctx,
			&oldPositions,
			getDistinctPositionsSQL,
			courseID,
		); err != nil {
			return fmt.Errorf("get positions: %w", err)
		}

		// Redistribute positions: position i gets the position at fraction
		// (i+1)/(total+1) of the alphabet range, leaving a gap at both ends
		// for future inserts.
		total := len(oldPositions)
		newPositions := make([]string, total)
		for i := range oldPositions {
			newPositions[i] = blocks.IndexToPosition(i+1, total+1)
		}

		if _, err := tx.ExecContext(
			ctx,
			remapPositionsSQL,
			courseID,
			pq.StringArray(oldPositions),
			pq.StringArray(newPositions),
		); err != nil {
			return fmt.Errorf("update block positions: %w", err)
		}
		return nil
	})
}

// DeleteBlockByID deletes the block in the actor's draft: a block that was
// never published loses its draft, a published one gets a draft version
// with deleted = true.
func (r *PGRepo) DeleteBlockByID(
	ctx context.Context,
	editCtx blocks.EditContext,
	ref blocks.BlockRef,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourse(ctx, tx, ref.CourseID); err != nil {
			return err
		}
		draft, err := ensureDraft(ctx, tx, ref.CourseID, ref.BlockID, editCtx.UserID, ref.Version)
		if err != nil {
			return err
		}

		if draft.BaseVersion == nil {
			zap.L().
				Debug("Executing query within transaction", zap.String("query", deleteBlockVersionSQL))
			if _, err := tx.ExecContext(ctx, deleteBlockVersionSQL, draft.ID, draft.Version); err != nil {
				return fmt.Errorf("tx delete draft: %w", err)
			}
		} else {
			zap.L().
				Debug("Executing query within transaction", zap.String("query", updateDraftDeletedSQL))
			if _, err := tx.ExecContext(ctx, updateDraftDeletedSQL, draft.ID, draft.Version, true); err != nil {
				return fmt.Errorf("tx delete block: %w", err)
			}
		}

		return deleteUnlinkedTasks(ctx, tx, []uuid.UUID{draft.ID})
	})
}

// deleteUnlinkedTasks soft-deletes the tasks of blockIDs that lost their
// last live block, and drops the task links of blocks with no version left.
func deleteUnlinkedTasks(
	ctx context.Context,
	tx *sqlx.Tx,
	blockIDs []uuid.UUID,
) error {
	if len(blockIDs) == 0 {
		return nil
	}
	ids := uuidArray(blockIDs)

	zap.L().
		Debug("Executing query within transaction", zap.String("query", deleteUnlinkedTasksSQL))

	if _, err := tx.ExecContext(ctx, deleteUnlinkedTasksSQL, ids); err != nil {
		return fmt.Errorf("tx delete unlinked tasks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteOrphanTaskBlocksSQL, ids); err != nil {
		return fmt.Errorf("tx delete orphan task blocks: %w", err)
	}
	return nil
}

// uuidArray passes a list of ids as a text[] the driver accepts.
func uuidArray(ids []uuid.UUID) pq.StringArray {
	out := make(pq.StringArray, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
