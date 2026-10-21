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
	"github.com/dsc-sgu/mm-backend/internal/checkpoints"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

// A task is read as its identity from tasks plus the version of its block
// the viewer sees, whose data holds the settings.
const taskColumns = `t.id, t.task_group_id, t.name, v.id AS block_id, v.version, v.draft, v.data`

var (
	// Only resolves a task if its block is published, so a git push can't
	// target a task that only exists in a draft.
	getTaskByNameSQL = `
		WITH v AS (` + blockView("(SELECT course_id FROM task_groups WHERE id = $1)", "NULL") + `)
		SELECT t.id
		FROM tasks t
		JOIN task_blocks tb ON tb.task_id = t.id
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE t.task_group_id = $1 AND t.name = $2 AND t.deleted_at IS NULL
	`

	// The published tasks of a group, the ones a git push may submit to.
	getLiveTasksSQL = `
		WITH v AS (` + blockView("(SELECT course_id FROM task_groups WHERE id = $1)", "NULL") + `)
		SELECT t.id, t.name, v.data->'patterns', (v.data->>'maxAttempts')::integer
		FROM tasks t
		JOIN task_blocks tb ON tb.task_id = t.id
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE t.task_group_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.name
	`

	// A group's tasks are the ones its live blocks link to, as the viewer
	// sees the course: their own drafts over the published blocks.
	getTasksByGroupSQL = `
		WITH v AS (` + blockView("(SELECT course_id FROM task_groups WHERE id = $1)", "$2") + `)
		SELECT ` + taskColumns + `
		FROM tasks t
		JOIN task_blocks tb ON tb.task_id = t.id
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE t.task_group_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.name
	`

	getTaskByIDSQL = `
		WITH v AS (` + blockView(taskCourseSQL, "$2") + `)
		SELECT ` + taskColumns + `
		FROM tasks t
		JOIN task_blocks tb ON tb.task_id = t.id
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`

	// The patterns a push is checked against are the published ones.
	getTaskPatternsByTaskIDSQL = `
		WITH v AS (` + blockView(taskCourseSQL, "NULL") + `)
		SELECT v.data->'patterns'
		FROM task_blocks tb
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE tb.task_id = $1
	`
)

const (
	getTaskGroupByIDSQL = `
		SELECT id, course_id, name FROM task_groups
		WHERE id = $1 AND deleted_at IS NULL
	`

	getTaskGroupByNameSQL = `
		SELECT id, course_id, name
		FROM task_groups
		WHERE name = $1 AND course_id = $2 AND deleted_at IS NULL
	`

	updateTaskGroupSQL = `
		UPDATE task_groups SET name = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING id, course_id, name
	`

	// The block and course of a live task, for editing it.
	getTaskBlockSQL = `
		SELECT tb.block_id, tg.course_id
		FROM tasks t
		JOIN task_groups tg ON tg.id = t.task_group_id
		JOIN task_blocks tb ON tb.task_id = t.id
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`

	getTaskGroupIDByNameSQL = `
		SELECT id FROM task_groups
		WHERE name = $1 AND course_id = $2 AND deleted_at IS NULL
	`

	getCourseIDByTaskGroupSQL = `
		SELECT course_id FROM task_groups
		WHERE id = $1 AND deleted_at IS NULL
	`

	getTaskCountSQL = `
		SELECT COUNT(*) FROM tasks WHERE task_group_id = $1 AND deleted_at IS NULL
	`
)

// taskRow is a task as taskColumns select it.
type taskRow struct {
	ID          uuid.UUID       `db:"id"`
	TaskGroupID uuid.UUID       `db:"task_group_id"`
	Name        string          `db:"name"`
	BlockID     uuid.UUID       `db:"block_id"`
	Version     int             `db:"version"`
	Draft       bool            `db:"draft"`
	Data        json.RawMessage `db:"data"`
}

func (row taskRow) task() (*tasks.Task, error) {
	var settings blocks.TaskSettings
	if err := json.Unmarshal(row.Data, &settings); err != nil {
		return nil, fmt.Errorf("decode task %s settings: %w", row.ID, err)
	}
	return &tasks.Task{
		ID:          row.ID,
		TaskGroupID: row.TaskGroupID,
		Name:        row.Name,
		BlockID:     row.BlockID,
		Version:     row.Version,
		Draft:       row.Draft,
		Statement:   settings.Statement,
		Patterns:    settings.Patterns,
		MaxGrade:    settings.MaxGrade,
		MaxAttempts: settings.MaxAttempts,
		AvailableAt: settings.AvailableAt,
		DeadlineAt:  settings.DeadlineAt,
	}, nil
}

func (r *PGRepo) GetTaskGroupIDByName(
	ctx context.Context,
	name string,
	courseID uuid.UUID,
) (uuid.UUID, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getTaskGroupIDByNameSQL))

	var groupID uuid.UUID
	err := r.db.GetContext(
		ctx,
		&groupID,
		getTaskGroupIDByNameSQL,
		name,
		courseID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf(
				"task group %q in course %s: not found",
				name,
				courseID,
			)
		}
		return uuid.Nil, fmt.Errorf("get task group id by name: %w", err)
	}
	return groupID, nil
}

func (r *PGRepo) GetTaskGroupByID(
	ctx context.Context,
	id uuid.UUID,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupByIDSQL))

	var tg tasks.TaskGroup
	err := r.db.GetContext(ctx, &tg, getTaskGroupByIDSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task group %s: %w", id, err)
	}
	return &tg, nil
}

func (r *PGRepo) GetTaskGroupByName(
	ctx context.Context,
	name string,
	courseID uuid.UUID,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupByNameSQL))

	var tg tasks.TaskGroup
	err := r.db.GetContext(ctx, &tg, getTaskGroupByNameSQL, name, courseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf(
				"task group %q in course %s: not found",
				name,
				courseID,
			)
		}
		return nil, fmt.Errorf("get task group by name: %w", err)
	}
	return &tg, nil
}

func (r *PGRepo) UpdateTaskGroup(
	ctx context.Context,
	id uuid.UUID,
	update *tasks.UpdateTaskGroup,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", updateTaskGroupSQL))

	var tg tasks.TaskGroup
	err := r.db.QueryRowContext(ctx, updateTaskGroupSQL, update.Name, id).
		Scan(&tg.ID, &tg.CourseID, &tg.Name)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return nil, tasks.ErrTaskGroupNameTaken
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task group %s: not found", id)
		}
		return nil, fmt.Errorf("update task group: %w", err)
	}
	return &tg, nil
}

func (r *PGRepo) GetTaskByID(
	ctx context.Context,
	taskID, viewerID uuid.UUID,
) (*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskByIDSQL))

	return getTask(ctx, r.db, taskID, viewerID)
}

// getTask reads a task as viewerID sees it with q, the connection or the
// transaction at hand.
func getTask(
	ctx context.Context,
	q sqlx.QueryerContext,
	taskID, viewerID uuid.UUID,
) (*tasks.Task, error) {
	var row taskRow
	err := sqlx.GetContext(ctx, q, &row, getTaskByIDSQL, taskID, viewerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, tasks.ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return row.task()
}

// UpdateTask puts the settings into userID's draft of the task's block.
// A draft this call creates is published at once, under the course row
// lock like any publish, and recorded as a checkpoint.
func (r *PGRepo) UpdateTask(
	ctx context.Context,
	taskID, userID uuid.UUID,
	update *tasks.TaskUpdate,
) (*tasks.UpdatedTask, error) {
	var result tasks.UpdatedTask

	err := r.ExecInTx(ctx, func(tx *sqlx.Tx) error {
		var ref struct {
			BlockID  uuid.UUID `db:"block_id"`
			CourseID uuid.UUID `db:"course_id"`
		}
		if err := tx.GetContext(ctx, &ref, getTaskBlockSQL, taskID); err != nil {
			if err == sql.ErrNoRows {
				return tasks.ErrTaskNotFound
			}
			return fmt.Errorf("get task block: %w", err)
		}
		if err := lockCourseForUpdate(ctx, tx, ref.CourseID); err != nil {
			return err
		}

		draft, created, err := ensureDraft(
			ctx,
			tx,
			ref.CourseID,
			ref.BlockID,
			userID,
			update.Version,
		)
		if err != nil {
			return err
		}

		var settings blocks.TaskSettings
		if err := json.Unmarshal(draft.Data, &settings); err != nil {
			return fmt.Errorf("decode task settings: %w", err)
		}
		applyTaskUpdate(&settings, update)
		data, err := json.Marshal(settings)
		if err != nil {
			return fmt.Errorf("encode task settings: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			updateDraftDataSQL,
			draft.ID,
			draft.Version,
			string(data),
		); err != nil {
			return fmt.Errorf("tx update task settings: %w", err)
		}

		if created {
			draft.Data = data
			outcome, err := publishDraft(ctx, tx, draft)
			if err != nil {
				return err
			}
			if outcome == publishConflict {
				return &checkpoints.ConflictError{BlockIDs: []uuid.UUID{draft.ID}}
			}
			if _, err := createCheckpoint(ctx, tx, ref.CourseID, userID); err != nil {
				return err
			}
		}

		task, err := getTask(ctx, tx, taskID, userID)
		if err != nil {
			return err
		}
		result = tasks.UpdatedTask{Task: *task, Published: created}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// applyTaskUpdate sets the fields the update carries. maxAttempts may be
// cleared with null, so whether it was sent is told apart from its value.
func applyTaskUpdate(settings *blocks.TaskSettings, update *tasks.TaskUpdate) {
	if len(update.Statement) > 0 {
		settings.Statement = update.Statement
	}
	if update.Patterns != nil {
		settings.Patterns = *update.Patterns
		if settings.Patterns == nil {
			settings.Patterns = []string{}
		}
	}
	if update.MaxGrade != nil {
		settings.MaxGrade = *update.MaxGrade
	}
	if update.MaxAttempts.Sent {
		settings.MaxAttempts = update.MaxAttempts.Ptr()
	}
	if update.AvailableAt != nil {
		settings.AvailableAt = update.AvailableAt
	}
	if update.DeadlineAt != nil {
		settings.DeadlineAt = update.DeadlineAt
	}
}

func (r *PGRepo) GetTasks(
	ctx context.Context,
	taskGroupID, viewerID uuid.UUID,
) ([]*tasks.Task, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getTasksByGroupSQL))

	var rows []taskRow
	if err := r.db.SelectContext(
		ctx,
		&rows,
		getTasksByGroupSQL,
		taskGroupID,
		viewerID,
	); err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}

	taskList := make([]*tasks.Task, 0, len(rows))
	for _, row := range rows {
		task, err := row.task()
		if err != nil {
			return nil, err
		}
		taskList = append(taskList, task)
	}
	return taskList, nil
}

func (r *PGRepo) GetTaskCount(
	ctx context.Context,
	taskGroupID uuid.UUID,
) (int, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskCountSQL))

	var count int
	err := r.db.GetContext(ctx, &count, getTaskCountSQL, taskGroupID)
	if err != nil {
		return 0, fmt.Errorf("get task count: %w", err)
	}
	return count, nil
}

func (r *PGRepo) GetCourseIDByTask(
	ctx context.Context,
	taskID uuid.UUID,
) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskBlockSQL))

	var ref struct {
		BlockID  uuid.UUID `db:"block_id"`
		CourseID uuid.UUID `db:"course_id"`
	}
	if err := r.db.GetContext(ctx, &ref, getTaskBlockSQL, taskID); err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, tasks.ErrTaskNotFound
		}
		return uuid.Nil, fmt.Errorf("get course by task %s: %w", taskID, err)
	}
	return ref.CourseID, nil
}

func (r *PGRepo) GetCourseIDByTaskGroup(
	ctx context.Context,
	taskGroupID uuid.UUID,
) (uuid.UUID, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getCourseIDByTaskGroupSQL))

	var courseID uuid.UUID
	err := r.db.GetContext(
		ctx,
		&courseID,
		getCourseIDByTaskGroupSQL,
		taskGroupID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, tasks.ErrTaskGroupNotFound
		}
		return uuid.Nil, fmt.Errorf(
			"get course by task group %s: %w",
			taskGroupID,
			err,
		)
	}
	return courseID, nil
}

// GetTaskByName resolves a published task of taskGroupID by name, the key a
// git push names it by.
func (r *PGRepo) GetTaskByName(
	ctx context.Context,
	taskGroupID uuid.UUID,
	name string,
) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskByNameSQL))

	var taskID uuid.UUID
	err := r.db.QueryRowContext(ctx, getTaskByNameSQL, taskGroupID, name).
		Scan(&taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf(
				"task %q in group %s: %w",
				name,
				taskGroupID,
				tasks.ErrTaskNotFound,
			)
		}
		return uuid.Nil, fmt.Errorf("get task by name: %w", err)
	}
	return taskID, nil
}

// GetTaskPatternsByTaskID returns the published file patterns of a task,
// nil for a task that is not published.
func (r *PGRepo) GetTaskPatternsByTaskID(
	ctx context.Context,
	taskID uuid.UUID,
) ([]string, error) {
	zap.L().
		Debug("Executing query", zap.String("query", getTaskPatternsByTaskIDSQL))

	var raw json.RawMessage
	err := r.db.GetContext(ctx, &raw, getTaskPatternsByTaskIDSQL, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task patterns by id: %w", err)
	}
	var patterns []string
	if err := json.Unmarshal(raw, &patterns); err != nil {
		return nil, fmt.Errorf("decode task %s patterns: %w", taskID, err)
	}
	return patterns, nil
}

// GetLiveTasks lists the published tasks of a task group, the ones a git
// push may submit to, with the patterns a submission is checked against.
func (r *PGRepo) GetLiveTasks(
	ctx context.Context,
	taskGroupID uuid.UUID,
) ([]pkggit.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getLiveTasksSQL))

	rows, err := r.db.QueryContext(ctx, getLiveTasksSQL, taskGroupID)
	if err != nil {
		return nil, fmt.Errorf("get live tasks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Warn("close live tasks rows", zap.Error(err))
		}
	}()

	var liveTasks []pkggit.Task
	for rows.Next() {
		var task pkggit.Task
		var patterns json.RawMessage
		if err := rows.Scan(&task.ID, &task.Name, &patterns, &task.MaxAttempts); err != nil {
			return nil, fmt.Errorf("scan live task: %w", err)
		}
		if err := json.Unmarshal(patterns, &task.Patterns); err != nil {
			return nil, fmt.Errorf("decode task %s patterns: %w", task.ID, err)
		}
		liveTasks = append(liveTasks, task)
	}
	return liveTasks, rows.Err()
}
