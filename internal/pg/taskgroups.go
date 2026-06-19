package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

const (
	createTaskGroupSQL = `
		INSERT INTO task_groups (block_id, name)
		VALUES ($1, $2)
	`

	getTaskGroupByBlockIDSQL = `
		SELECT block_id, name FROM task_groups WHERE block_id = $1
	`

	getTaskGroupByNameSQL = `
		SELECT tg.block_id, tg.name
		FROM task_groups tg
		JOIN blocks b ON b.id = tg.block_id
		WHERE tg.name = $1 AND b.course_id = $2
	`

	updateTaskGroupSQL = `
		UPDATE task_groups SET name = $1 WHERE block_id = $2
		RETURNING block_id, name
	`

	deleteTaskGroupSQL = `
		DELETE FROM task_groups WHERE block_id = $1
	`

	createTaskSQL = `
		INSERT INTO tasks (task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time)
		VALUES ($1, (SELECT COALESCE(MAX(position), 0) + 1 FROM tasks WHERE task_group_id = $1), $2, $3, $4, $5, $6, $7)
		RETURNING id, task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time
	`

	getTaskByIDSQL = `
		SELECT id, task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time
		FROM tasks WHERE id = $1
	`

	getTaskByPositionSQL = `
		SELECT id, task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time
		FROM tasks WHERE task_group_id = $1 AND position = $2
	`

	getTasksSQL = `
		SELECT id, task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time
		FROM tasks WHERE task_group_id = $1 ORDER BY position
	`

	updateTaskSQL = `
		UPDATE tasks
		SET data = COALESCE($1, data),
		    max_grade = COALESCE($2, max_grade),
		    max_attempts = COALESCE($3, max_attempts),
		    available_at = COALESCE($4, available_at),
		    deadline_at = COALESCE($5, deadline_at),
		    lead_time = COALESCE($6, lead_time)
		WHERE id = $7
		RETURNING id, task_group_id, position, data, max_grade, max_attempts, available_at, deadline_at, lead_time
	`

	deleteTaskSQL = `
		DELETE FROM tasks WHERE id = $1
	`

	getTaskGroupIDByNameSQL = `
		SELECT tg.block_id
		FROM task_groups tg
		JOIN blocks b ON b.id = tg.block_id
		WHERE tg.name = $1 AND b.course_id = $2
	`

	getCourseIDByTaskGroupSQL = `
		SELECT b.course_id
		FROM blocks b
		JOIN task_groups tg ON tg.block_id = b.id
		WHERE tg.block_id = $1
	`

	getTaskIDByPositionSQL = `
		SELECT id FROM tasks WHERE task_group_id = $1 AND position = $2
	`

	getTaskCountSQL = `
		SELECT COUNT(*) FROM tasks WHERE task_group_id = $1
	`

	hasSubmittedAttemptSQL = `
		SELECT EXISTS(
			SELECT 1 FROM attempts a
			JOIN tasks t ON a.task_id = t.id
			JOIN attempt_transitions att ON att.attempt_id = a.id
			WHERE a.user_id = $1 AND t.task_group_id = $2 AND t.position = $3 AND att.state = 'submitted'
		)
	`
)

func (r *PGRepo) GetTaskGroupIDByName(ctx context.Context, name string, courseID uuid.UUID) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupIDByNameSQL))

	var blockID uuid.UUID
	err := r.db.GetContext(ctx, &blockID, getTaskGroupIDByNameSQL, name, courseID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task group %q in course %s: not found", name, courseID)
		}
		return uuid.Nil, fmt.Errorf("get task group id by name: %w", err)
	}
	return blockID, nil
}

func (r *PGRepo) CreateTaskGroup(
	ctx context.Context,
	blockID uuid.UUID,
	model *tasks.CreateTaskGroup,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", createTaskGroupSQL))

	if _, err := r.db.ExecContext(ctx, createTaskGroupSQL, blockID, model.Name); err != nil {
		return nil, fmt.Errorf("create task group: %w", err)
	}

	return &tasks.TaskGroup{
		BlockID: blockID,
		Name:    model.Name,
	}, nil
}

func (r *PGRepo) GetTaskGroupByBlockID(ctx context.Context, blockID uuid.UUID) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskGroupByBlockIDSQL))

	var tg tasks.TaskGroup
	err := r.db.GetContext(ctx, &tg, getTaskGroupByBlockIDSQL, blockID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task group by block %s: %w", blockID, err)
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
	blockID uuid.UUID,
	update *tasks.UpdateTaskGroup,
) (*tasks.TaskGroup, error) {
	zap.L().Debug("Executing query", zap.String("query", updateTaskGroupSQL))

	var tg tasks.TaskGroup
	err := r.db.QueryRowContext(ctx, updateTaskGroupSQL, *update.Name, blockID).Scan(&tg.BlockID, &tg.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task group %s: not found", blockID)
		}
		return nil, fmt.Errorf("update task group: %w", err)
	}
	return &tg, nil
}

func (r *PGRepo) DeleteTaskGroup(ctx context.Context, blockID uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteTaskGroupSQL))

	_, err := r.db.ExecContext(ctx, deleteTaskGroupSQL, blockID)
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
		model.TaskGroupID, model.Data, model.MaxGrade, model.MaxAttempts,
		model.AvailableAt, model.DeadlineAt, model.LeadTime,
	).Scan(&task.ID, &task.TaskGroupID, &task.Position, &task.Data,
		&task.MaxGrade, &task.MaxAttempts, &task.AvailableAt, &task.DeadlineAt, &task.LeadTime)
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

func (r *PGRepo) GetTaskByPosition(ctx context.Context, taskGroupID uuid.UUID, position int) (*tasks.Task, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskByPositionSQL))

	var task tasks.Task
	err := r.db.GetContext(ctx, &task, getTaskByPositionSQL, taskGroupID, position)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task at position %d in group %s: not found", position, taskGroupID)
		}
		return nil, fmt.Errorf("get task by position: %w", err)
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

	var task tasks.Task
	err := r.db.QueryRowContext(
		ctx, updateTaskSQL,
		update.Data, update.MaxGrade, update.MaxAttempts,
		update.AvailableAt, update.DeadlineAt, update.LeadTime, taskID,
	).Scan(&task.ID, &task.TaskGroupID, &task.Position, &task.Data,
		&task.MaxGrade, &task.MaxAttempts, &task.AvailableAt, &task.DeadlineAt, &task.LeadTime)
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

func (r *PGRepo) GetTaskIDByPosition(ctx context.Context, taskGroupID uuid.UUID, position int) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskIDByPositionSQL))

	var taskID uuid.UUID
	err := r.db.GetContext(ctx, &taskID, getTaskIDByPositionSQL, taskGroupID, position)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task at position %d in group %s: not found", position, taskGroupID)
		}
		return uuid.Nil, fmt.Errorf("get task id by position: %w", err)
	}
	return taskID, nil
}

func (r *PGRepo) HasSubmittedAttempt(
	ctx context.Context,
	userID uuid.UUID,
	taskGroupID uuid.UUID,
	position int,
) (bool, error) {
	zap.L().Debug("Executing query", zap.String("query", hasSubmittedAttemptSQL))

	var exists bool
	err := r.db.GetContext(ctx, &exists, hasSubmittedAttemptSQL, userID, taskGroupID, position)
	if err != nil {
		return false, fmt.Errorf("has submitted attempt: %w", err)
	}
	return exists, nil
}
