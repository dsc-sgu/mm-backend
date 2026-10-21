package pg

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/checkpoints"
)

const (
	getOwnDraftsSQL = `
		SELECT ` + rawBlockColumns + `
		FROM blocks
		WHERE course_id = $1 AND draft AND created_by = $2
		ORDER BY id
	`

	publishDraftInPlaceSQL = `
		UPDATE blocks SET draft = false, base_version = NULL
		WHERE id = $1 AND version = $2
	`

	deleteOwnDraftsSQL = `
		DELETE FROM blocks
		WHERE course_id = $1 AND draft AND created_by = $2
		RETURNING id
	`

	createCheckpointSQL = `
		INSERT INTO checkpoints (course_id, created_by, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id, course_id, created_by, created_at
	`

	getCheckpointsSQL = `
		SELECT id, course_id, created_by, created_at
		FROM checkpoints
		WHERE course_id = $1
		ORDER BY created_at DESC, id DESC
	`

	getCheckpointSQL = `
		SELECT id, course_id, created_by, created_at
		FROM checkpoints
		WHERE id = $1 AND course_id = $2
	`

	getCheckpointBlocksSQL = `
		SELECT b.id, b.version, b.course_id, b.block_type,
		       COALESCE(b.data, jsonb_build_object('version', 1, 'taskId', tb.task_id)) AS data,
		       b.position, b.draft, b.deleted, b.created_by, b.base_version
		FROM checkpoint_blocks cb
		JOIN blocks b ON b.id = cb.block_id AND b.version = cb.version
		LEFT JOIN task_blocks tb ON tb.block_id = b.id
		WHERE cb.checkpoint_id = $1
		ORDER BY b.position ASC, b.id ASC
	`

	// Brings the tasks of restored task blocks back, with their groups.
	undeleteTasksSQL = `
		WITH undeleted AS (
			UPDATE tasks t
			SET deleted_at = NULL
			FROM task_blocks tb
			WHERE tb.task_id = t.id
			  AND tb.block_id = ANY($1::uuid[])
			  AND t.deleted_at IS NOT NULL
			RETURNING t.task_group_id
		)
		UPDATE task_groups g
		SET deleted_at = NULL
		WHERE g.id IN (SELECT task_group_id FROM undeleted)
		  AND g.deleted_at IS NOT NULL
	`
)

var (
	fillCheckpointSQL = `
		WITH v AS (` + blockView("$2", "NULL") + `)
		INSERT INTO checkpoint_blocks (checkpoint_id, block_id, version)
		SELECT $1::uuid, id, version FROM v WHERE NOT deleted
	`

	// Republishes the block versions of checkpoint $1 whose content differs
	// from the block's current published head (or whose block is deleted or
	// never existed as published), as new versions by $2.
	restoreCheckpointBlocksSQL = `
		WITH head AS (` + blockView("$3", "NULL") + `)
		INSERT INTO blocks (id, version, course_id, block_type, data, position, draft, deleted, created_by, base_version)
		SELECT cb.block_id,
		       (SELECT COALESCE(MAX(version), 0) + 1 FROM blocks WHERE id = cb.block_id),
		       src.course_id, src.block_type, src.data, src.position, false, false, $2::uuid, NULL
		FROM checkpoint_blocks cb
		JOIN blocks src ON src.id = cb.block_id AND src.version = cb.version
		LEFT JOIN head h ON h.id = cb.block_id
		WHERE cb.checkpoint_id = $1
		  AND (
			h.id IS NULL
			OR h.deleted
			OR h.block_type <> src.block_type
			OR h.data IS DISTINCT FROM src.data
			OR h.position <> src.position
		  )
		RETURNING id
	`

	// Deletes, as new versions by $2, the live blocks of course $3 that
	// checkpoint $1 does not contain.
	tombstoneBlocksSQL = `
		WITH head AS (` + blockView("$3", "NULL") + `)
		INSERT INTO blocks (id, version, course_id, block_type, data, position, draft, deleted, created_by, base_version)
		SELECT h.id,
		       (SELECT COALESCE(MAX(version), 0) + 1 FROM blocks WHERE id = h.id),
		       h.course_id, h.block_type, h.data, h.position, false, true, $2::uuid, NULL
		FROM head h
		WHERE NOT h.deleted
		  AND NOT EXISTS (
			SELECT 1 FROM checkpoint_blocks cb
			WHERE cb.checkpoint_id = $1 AND cb.block_id = h.id
		  )
		RETURNING id
	`
)

// PublishDrafts publishes every draft of userID in the course in one
// transaction, under the course row lock so publishes of one course run
// one at a time.
func (r *PGRepo) PublishDrafts(
	ctx context.Context,
	courseID, userID uuid.UUID,
) (*checkpoints.Checkpoint, error) {
	var checkpoint *checkpoints.Checkpoint

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourseForUpdate(ctx, tx, courseID); err != nil {
			return err
		}

		var drafts []*blocks.Block
		if err := tx.SelectContext(ctx, &drafts, getOwnDraftsSQL, courseID, userID); err != nil {
			return fmt.Errorf("get drafts: %w", err)
		}
		if len(drafts) == 0 {
			return checkpoints.ErrNothingToPublish
		}

		var conflicts, deleted []uuid.UUID
		for _, draft := range drafts {
			outcome, err := publishDraft(ctx, tx, draft)
			if err != nil {
				return err
			}
			switch outcome {
			case publishConflict:
				conflicts = append(conflicts, draft.ID)
			case publishDeleted:
				deleted = append(deleted, draft.ID)
			}
		}
		if len(conflicts) > 0 {
			return &checkpoints.ConflictError{BlockIDs: conflicts}
		}

		if err := deleteUnlinkedTasks(ctx, tx, deleted); err != nil {
			return err
		}

		var err error
		checkpoint, err = createCheckpoint(ctx, tx, courseID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return checkpoint, nil
}

type publishOutcome int

const (
	publishLive publishOutcome = iota
	publishDeleted
	publishConflict
)

// publishDraft makes one draft published. A draft of a never-published
// block, or of a block nobody published since its base version, becomes
// the published version as it is. Otherwise base, head and draft are
// compared field by field (see merge) and the result is a new version.
func publishDraft(
	ctx context.Context,
	tx *sqlx.Tx,
	draft *blocks.Block,
) (publishOutcome, error) {
	if _, err := tx.ExecContext(ctx, lockBlockSQL, draft.ID); err != nil {
		return 0, fmt.Errorf("lock block: %w", err)
	}

	if draft.BaseVersion == nil {
		if draft.Deleted {
			if err := deleteBlockVersion(ctx, tx, draft); err != nil {
				return 0, err
			}
			return publishDeleted, nil
		}
		return publishLive, publishInPlace(ctx, tx, draft)
	}

	head, err := getHead(ctx, tx, draft.ID)
	if err != nil {
		return 0, err
	}
	if head == nil {
		return 0, fmt.Errorf("block %s: draft has a base but no published version", draft.ID)
	}

	if head.Version == *draft.BaseVersion {
		if err := publishInPlace(ctx, tx, draft); err != nil {
			return 0, err
		}
		if draft.Deleted {
			return publishDeleted, nil
		}
		return publishLive, nil
	}

	var base blocks.Block
	if err := tx.GetContext(ctx, &base, getBlockVersionSQL, draft.ID, *draft.BaseVersion); err != nil {
		return 0, fmt.Errorf("get base version: %w", err)
	}

	merged, ok := merge(&base, head, draft)
	if !ok {
		return publishConflict, nil
	}
	if err := deleteBlockVersion(ctx, tx, draft); err != nil {
		return 0, err
	}
	if merged == nil {
		// Deleted on both sides: the head already says so
		return publishDeleted, nil
	}

	version, err := nextVersion(ctx, tx, draft.ID)
	if err != nil {
		return 0, err
	}
	merged.Version = version
	merged.Draft = false
	merged.CreatedBy = draft.CreatedBy
	merged.BaseVersion = nil
	if err := insertBlock(ctx, tx, merged); err != nil {
		return 0, err
	}
	if merged.Deleted {
		return publishDeleted, nil
	}
	return publishLive, nil
}

// merge reconciles a draft with the head published after its base, field
// by field: a field changed on one side only takes that side's value; the
// position changed on both takes the draft's; the data changed on both, or
// a deletion against any change, is a conflict. It returns nil, true when
// both sides deleted the block, and false on a conflict.
func merge(base, head, draft *blocks.Block) (*blocks.Block, bool) {
	posTheirs := head.Position != base.Position
	posMine := draft.Position != base.Position
	dataTheirs := !bytes.Equal(head.Data, base.Data)
	dataMine := !bytes.Equal(draft.Data, base.Data)

	switch {
	case head.Deleted && draft.Deleted:
		return nil, true
	case head.Deleted:
		if posMine || dataMine {
			return nil, false
		}
		return nil, true
	case draft.Deleted:
		if posTheirs || dataTheirs {
			return nil, false
		}
		result := *head
		result.Deleted = true
		return &result, true
	}

	if dataMine && dataTheirs {
		return nil, false
	}
	result := *head
	if dataMine {
		result.Data = draft.Data
	}
	if posMine {
		result.Position = draft.Position
	}
	return &result, true
}

func publishInPlace(ctx context.Context, tx *sqlx.Tx, draft *blocks.Block) error {
	if _, err := tx.ExecContext(ctx, publishDraftInPlaceSQL, draft.ID, draft.Version); err != nil {
		return fmt.Errorf("publish draft: %w", err)
	}
	return nil
}

func deleteBlockVersion(ctx context.Context, tx *sqlx.Tx, b *blocks.Block) error {
	if _, err := tx.ExecContext(ctx, deleteBlockVersionSQL, b.ID, b.Version); err != nil {
		return fmt.Errorf("delete block version: %w", err)
	}
	return nil
}

// createCheckpoint records the published course as it is now.
func createCheckpoint(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID, userID uuid.UUID,
) (*checkpoints.Checkpoint, error) {
	var checkpoint checkpoints.Checkpoint
	if err := tx.GetContext(ctx, &checkpoint, createCheckpointSQL, courseID, userID); err != nil {
		return nil, fmt.Errorf("create checkpoint: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fillCheckpointSQL, checkpoint.ID, courseID); err != nil {
		return nil, fmt.Errorf("fill checkpoint: %w", err)
	}
	return &checkpoint, nil
}

// DiscardDrafts deletes every draft of userID in the course, with the
// tasks that only those drafts linked to.
func (r *PGRepo) DiscardDrafts(
	ctx context.Context,
	courseID, userID uuid.UUID,
) error {
	return r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourse(ctx, tx, courseID); err != nil {
			return err
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", deleteOwnDraftsSQL))

		var ids []uuid.UUID
		if err := tx.SelectContext(ctx, &ids, deleteOwnDraftsSQL, courseID, userID); err != nil {
			return fmt.Errorf("delete drafts: %w", err)
		}
		return deleteUnlinkedTasks(ctx, tx, ids)
	})
}

func (r *PGRepo) GetCheckpoints(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*checkpoints.Checkpoint, error) {
	zap.L().Debug("Executing query", zap.String("query", getCheckpointsSQL))

	list := make([]*checkpoints.Checkpoint, 0)
	if err := r.db.SelectContext(ctx, &list, getCheckpointsSQL, courseID); err != nil {
		return nil, fmt.Errorf("get checkpoints: %w", err)
	}
	return list, nil
}

func (r *PGRepo) GetCheckpointBlocks(
	ctx context.Context,
	courseID, checkpointID uuid.UUID,
) ([]*blocks.Block, error) {
	var checkpoint checkpoints.Checkpoint
	if err := r.db.GetContext(ctx, &checkpoint, getCheckpointSQL, checkpointID, courseID); err != nil {
		if err == sql.ErrNoRows {
			return nil, checkpoints.ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}

	zap.L().Debug("Executing query", zap.String("query", getCheckpointBlocksSQL))

	blockList := make([]*blocks.Block, 0)
	if err := r.db.SelectContext(ctx, &blockList, getCheckpointBlocksSQL, checkpointID); err != nil {
		return nil, fmt.Errorf("get checkpoint blocks: %w", err)
	}
	return blockList, nil
}

// RestoreCheckpoint publishes the course as it was at the checkpoint, under
// the course row lock like any publish.
func (r *PGRepo) RestoreCheckpoint(
	ctx context.Context,
	courseID, checkpointID, userID uuid.UUID,
) (*checkpoints.Checkpoint, error) {
	var checkpoint *checkpoints.Checkpoint

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockCourseForUpdate(ctx, tx, courseID); err != nil {
			return err
		}

		var target checkpoints.Checkpoint
		if err := tx.GetContext(ctx, &target, getCheckpointSQL, checkpointID, courseID); err != nil {
			if err == sql.ErrNoRows {
				return checkpoints.ErrCheckpointNotFound
			}
			return fmt.Errorf("get checkpoint: %w", err)
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", restoreCheckpointBlocksSQL))

		var restored []uuid.UUID
		if err := tx.SelectContext(
			ctx,
			&restored,
			restoreCheckpointBlocksSQL,
			checkpointID,
			userID,
			courseID,
		); err != nil {
			return fmt.Errorf("restore blocks: %w", err)
		}

		zap.L().
			Debug("Executing query within transaction", zap.String("query", tombstoneBlocksSQL))

		var removed []uuid.UUID
		if err := tx.SelectContext(
			ctx,
			&removed,
			tombstoneBlocksSQL,
			checkpointID,
			userID,
			courseID,
		); err != nil {
			return fmt.Errorf("delete blocks added since: %w", err)
		}

		if len(restored) > 0 {
			_, err := tx.ExecContext(ctx, undeleteTasksSQL, uuidArray(restored))
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				return checkpoints.ErrTaskGroupNameTaken
			}
			if err != nil {
				return fmt.Errorf("undelete tasks: %w", err)
			}
		}
		if err := deleteUnlinkedTasks(ctx, tx, removed); err != nil {
			return err
		}

		var err error
		checkpoint, err = createCheckpoint(ctx, tx, courseID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return checkpoint, nil
}
