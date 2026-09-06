package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

const (
	// Scoped to the active snapshot, same as getTaskByNameSQL: with each
	// snapshot generation of a task keeping its own row, an unscoped query
	// here would mix in patterns from historical, no-longer-live generations.
	getTaskPatternsSQL = `
		SELECT t.name, t.patterns
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id AND b.deleted_at IS NULL
		JOIN course_snapshots cs ON cs.id = t.snapshot_id
		JOIN courses c ON c.id = cs.course_id AND c.active_snapshot_id = t.snapshot_id
		WHERE t.task_group_id = $1 AND array_length(t.patterns, 1) > 0
		ORDER BY t.name
	`

	// Only resolves a task if its block is part of the course's currently
	// active (published) snapshot, so a git push can't target a task that
	// only exists in an unpublished draft.
	getTaskByNameSQL = `
		SELECT t.block_id
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id AND b.deleted_at IS NULL
		JOIN course_snapshots cs ON cs.id = b.snapshot_id
		JOIN courses c ON c.id = cs.course_id AND c.active_snapshot_id = b.snapshot_id
		WHERE t.task_group_id = $1 AND t.name = $2
	`

	getTaskPatternsByTaskIDSQL = `
		SELECT patterns FROM tasks WHERE block_id = $1
	`

	createTaskGroupSQL = `
		INSERT INTO task_groups (course_id, name)
		VALUES ($1, $2)
		RETURNING id, course_id, name
	`

	getTaskGroupByIDSQL = `
		SELECT id, course_id, name FROM task_groups WHERE id = $1
	`

	getTaskGroupByNameSQL = `
		SELECT id, course_id, name
		FROM task_groups
		WHERE name = $1 AND course_id = $2
	`

	updateTaskGroupSQL = `
		UPDATE task_groups SET name = $1 WHERE id = $2
		RETURNING id, course_id, name
	`

	deleteTaskGroupSQL = `
		DELETE FROM task_groups WHERE id = $1
	`

	// Both queries join blocks so a task whose block has been (soft-)deleted
	// disappears from every read path, the same as a fully deleted task would.
	getTaskByIDSQL = `
		SELECT t.block_id, t.task_group_id, t.name, t.patterns, t.max_grade, t.max_attempts, t.available_at, t.deadline_at
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id AND b.deleted_at IS NULL
		WHERE t.block_id = $1
	`

	// Scoped to a specific snapshot (the caller's own in-progress draft, or
	// the course's active/published snapshot — see ResolveViewSnapshot):
	// each snapshot generation of a task has its own row, so listing "all"
	// tasks for a group without this scope would return one row per
	// generation ever copied.
	getTasksByGroupAndSnapshotSQL = `
		SELECT t.block_id, t.task_group_id, t.name, t.patterns, t.max_grade, t.max_attempts, t.available_at, t.deadline_at
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id AND b.deleted_at IS NULL
		WHERE t.task_group_id = $1 AND t.snapshot_id = $2
		ORDER BY t.name
	`

	// resolveViewSnapshotSQL picks the caller's own in-progress draft for
	// courseID, if they currently hold its edit lock; ResolveViewSnapshot
	// falls back to the course's active snapshot when this finds nothing.
	resolveViewSnapshotSQL = `
		SELECT cs.id
		FROM course_snapshots cs
		JOIN course_locks cl ON cl.course_id = cs.course_id
		WHERE cs.course_id = $1
		  AND cs.status = 'draft'
		  AND cs.created_by = $2
		  AND cl.user_id = $2
		  AND cl.session_id = $3
		  AND cl.expires_at > NOW()
		LIMIT 1
	`

	updateTaskSQL = `
		UPDATE tasks
		SET patterns = COALESCE($1::text[], patterns),
		    max_grade = COALESCE($2, max_grade),
		    max_attempts = COALESCE($3, max_attempts),
		    available_at = COALESCE($4, available_at),
		    deadline_at = COALESCE($5, deadline_at)
		WHERE block_id = $6
		RETURNING block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at
	`

	getTaskGroupIDByNameSQL = `
		SELECT id FROM task_groups WHERE name = $1 AND course_id = $2
	`

	getCourseIDByTaskGroupSQL = `
		SELECT course_id FROM task_groups WHERE id = $1
	`

	getTaskCountSQL = `
		SELECT COUNT(*) FROM tasks WHERE task_group_id = $1
	`
)

func (r *PGRepo) GetTaskGroupIDByName(ctx context.Context, name string, courseID uuid.UUID) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupIDByNameSQL))

	var groupID uuid.UUID
	err := r.db.GetContext(ctx, &groupID, getTaskGroupIDByNameSQL, name, courseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task group %q in course %s: not found", name, courseID)
		}
		return uuid.Nil, fmt.Errorf("get task group id by name: %w", err)
	}
	return groupID, nil
}

func (r *PGRepo) CreateTaskGroup(
	ctx context.Context,
	model *tasks.CreateTaskGroup,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", createTaskGroupSQL))

	var tg tasks.TaskGroup
	err := r.db.QueryRowContext(ctx, createTaskGroupSQL, model.CourseID, model.Name).
		Scan(&tg.ID, &tg.CourseID, &tg.Name)
	if err != nil {
		return nil, fmt.Errorf("create task group: %w", err)
	}
	return &tg, nil
}

func (r *PGRepo) GetTaskGroupByID(ctx context.Context, id uuid.UUID) (*tasks.TaskGroup, error) {
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

func (r *PGRepo) GetTaskGroupByName(ctx context.Context, name string, courseID uuid.UUID) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupByNameSQL))

	var tg tasks.TaskGroup
	err := r.db.GetContext(ctx, &tg, getTaskGroupByNameSQL, name, courseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task group %q in course %s: not found", name, courseID)
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
	err := r.db.QueryRowContext(ctx, updateTaskGroupSQL, *update.Name, id).
		Scan(&tg.ID, &tg.CourseID, &tg.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task group %s: not found", id)
		}
		return nil, fmt.Errorf("update task group: %w", err)
	}
	return &tg, nil
}

func (r *PGRepo) DeleteTaskGroup(ctx context.Context, id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteTaskGroupSQL))

	_, err := r.db.ExecContext(ctx, deleteTaskGroupSQL, id)
	if err != nil {
		return fmt.Errorf("delete task group: %w", err)
	}
	return nil
}

func (r *PGRepo) GetTaskByID(ctx context.Context, taskID uuid.UUID) (*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskByIDSQL))

	var task tasks.Task
	err := r.db.GetContext(ctx, &task, getTaskByIDSQL, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task %s: not found", taskID)
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return &task, nil
}

func (r *PGRepo) GetTasks(ctx context.Context, taskGroupID, snapshotID uuid.UUID) ([]*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getTasksByGroupAndSnapshotSQL))

	rows, err := r.db.QueryxContext(ctx, getTasksByGroupAndSnapshotSQL, taskGroupID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Warn("close rows", zap.Error(err))
		}
	}()

	var taskList []*tasks.Task
	for rows.Next() {
		var task tasks.Task
		if err := rows.StructScan(&task); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		taskList = append(taskList, &task)
	}
	return taskList, rows.Err()
}

func (r *PGRepo) GetTaskCount(ctx context.Context, taskGroupID uuid.UUID) (int, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskCountSQL))

	var count int
	err := r.db.GetContext(ctx, &count, getTaskCountSQL, taskGroupID)
	if err != nil {
		return 0, fmt.Errorf("get task count: %w", err)
	}
	return count, nil
}

func (r *PGRepo) GetCourseIDByTaskGroup(ctx context.Context, taskGroupID uuid.UUID) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getCourseIDByTaskGroupSQL))

	var courseID uuid.UUID
	err := r.db.GetContext(ctx, &courseID, getCourseIDByTaskGroupSQL, taskGroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, tasks.ErrTaskGroupNotFound
		}
		return uuid.Nil, fmt.Errorf("get course by task group %s: %w", taskGroupID, err)
	}
	return courseID, nil
}

func (r *PGRepo) GetTaskPatterns(ctx context.Context, taskGroupID uuid.UUID) (map[string][]string, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskPatternsSQL))

	rows, err := r.db.QueryxContext(ctx, getTaskPatternsSQL, taskGroupID)
	if err != nil {
		return nil, fmt.Errorf("get task patterns: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Warn("close task patterns rows", zap.Error(err))
		}
	}()

	result := make(map[string][]string)
	for rows.Next() {
		var name string
		var patterns pq.StringArray
		if err := rows.Scan(&name, &patterns); err != nil {
			return nil, fmt.Errorf("scan task patterns: %w", err)
		}
		if len(patterns) > 0 {
			result[name] = patterns
		}
	}
	return result, rows.Err()
}

func (r *PGRepo) GetTaskByName(
	ctx context.Context,
	taskGroupID uuid.UUID,
	name string,
) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskByNameSQL))

	var taskID uuid.UUID
	err := r.db.QueryRowContext(ctx, getTaskByNameSQL, taskGroupID, name).Scan(&taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task %q in group %s: not found", name, taskGroupID)
		}
		return uuid.Nil, fmt.Errorf("get task by name: %w", err)
	}
	return taskID, nil
}

func (r *PGRepo) GetTaskPatternsByTaskID(ctx context.Context, taskID uuid.UUID) ([]string, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskPatternsByTaskIDSQL))

	var patterns pq.StringArray
	err := r.db.GetContext(ctx, &patterns, getTaskPatternsByTaskIDSQL, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task patterns by id: %w", err)
	}
	return patterns, nil
}

// ResolveViewSnapshot picks which snapshot generation of a course's tasks a
// caller should see: their own in-progress draft, if they currently hold the
// course's edit lock, otherwise the course's active (published) snapshot.
func (r *PGRepo) ResolveViewSnapshot(ctx context.Context, courseID, userID, sessionID uuid.UUID) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", resolveViewSnapshotSQL))

	var draftID uuid.UUID
	err := r.db.GetContext(ctx, &draftID, resolveViewSnapshotSQL, courseID, userID, sessionID)
	if err == nil {
		return draftID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("resolve draft snapshot: %w", err)
	}

	course, err := r.GetCourseByID(ctx, courseID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve view snapshot: get course: %w", err)
	}
	if course == nil || course.ActiveSnapshotID == nil {
		return uuid.Nil, fmt.Errorf("course %s has no active snapshot", courseID)
	}
	return *course.ActiveSnapshotID, nil
}
