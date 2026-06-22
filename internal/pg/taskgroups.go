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
	getTaskPatternsSQL = `
		SELECT name, patterns FROM tasks
		WHERE task_group_id = $1 AND array_length(patterns, 1) > 0
		ORDER BY name
	`

	getTaskByNameSQL = `
		SELECT block_id FROM tasks WHERE task_group_id = $1 AND name = $2
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

	createTaskSQL = `
		INSERT INTO tasks (block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at
	`

	getTaskByIDSQL = `
		SELECT block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at
		FROM tasks WHERE block_id = $1
	`

	getTasksSQL = `
		SELECT block_id, task_group_id, name, patterns, max_grade, max_attempts, available_at, deadline_at
		FROM tasks WHERE task_group_id = $1 ORDER BY name
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

	deleteTaskSQL = `
		DELETE FROM tasks WHERE block_id = $1
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

func (r *PGRepo) CreateTask(ctx context.Context, model *tasks.CreateTask) (*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", createTaskSQL))

	var task tasks.Task
	err := r.db.QueryRowContext(
		ctx, createTaskSQL,
		model.BlockID, model.TaskGroupID, model.Name, pq.StringArray(model.Patterns),
		model.MaxGrade, model.MaxAttempts, model.AvailableAt, model.DeadlineAt,
	).Scan(&task.ID, &task.TaskGroupID, &task.Name, &task.Patterns,
		&task.MaxGrade, &task.MaxAttempts, &task.AvailableAt, &task.DeadlineAt)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return &task, nil
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

func (r *PGRepo) GetTasks(ctx context.Context, taskGroupID uuid.UUID) ([]*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getTasksSQL))

	rows, err := r.db.QueryxContext(ctx, getTasksSQL, taskGroupID)
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

func (r *PGRepo) UpdateTask(ctx context.Context, taskID uuid.UUID, update *tasks.UpdateTask) (*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", updateTaskSQL))

	var patterns any
	if update.Patterns != nil {
		patterns = pq.StringArray(*update.Patterns)
	}

	var task tasks.Task
	err := r.db.QueryRowContext(
		ctx, updateTaskSQL,
		patterns, update.MaxGrade, update.MaxAttempts,
		update.AvailableAt, update.DeadlineAt, taskID,
	).Scan(&task.ID, &task.TaskGroupID, &task.Name, &task.Patterns,
		&task.MaxGrade, &task.MaxAttempts, &task.AvailableAt, &task.DeadlineAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task %s: not found", taskID)
		}
		return nil, fmt.Errorf("update task: %w", err)
	}
	return &task, nil
}

func (r *PGRepo) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteTaskSQL))

	_, err := r.db.ExecContext(ctx, deleteTaskSQL, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
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
