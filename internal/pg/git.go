package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
	"github.com/dsc-sgu/mm-backend/internal/auth/sshkeys"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

const (
	addSSHKeySQL = `
		INSERT INTO ssh_keys (owner_id, name, key, fingerprint, created_at)
		VALUES (:owner_id, :name, :key, :fingerprint, :created_at)
	`

	deleteSSHKeySQL = `
		DELETE FROM ssh_keys
		WHERE owner_id = $1 AND fingerprint = $2
	`

	getParticipantIDSQL = `
		SELECT owner_id FROM ssh_keys
		WHERE fingerprint = $1
	`

	repoForPathSQL = `
		SELECT c.id, tg.id, k.owner_id
		FROM courses c
		JOIN task_groups tg ON tg.course_id = c.id AND tg.name = $2 AND tg.deleted_at IS NULL
		JOIN ssh_keys k ON k.fingerprint = $3
		WHERE c.name = $1 AND c.deleted_at IS NULL
	`

	saveAttemptSQL = `
		WITH new_attempt AS (
			INSERT INTO attempts (user_id, task_id)
			VALUES ($1, $2)
			RETURNING id )
		INSERT INTO attempt_transitions (attempt_id, state, transition_at, transition_data)
		VALUES ((SELECT id FROM new_attempt), 'submitted', $3, $4::jsonb)
	`

	getAttemptCommitInfoSQL = `
		SELECT a.user_id, a.task_id, att.transition_data, t.task_group_id, tg.course_id
		FROM attempts a
		JOIN attempt_transitions att ON att.attempt_id = a.id
		JOIN tasks t ON a.task_id = t.id
		JOIN task_groups tg ON t.task_group_id = tg.id
		WHERE a.id = $1 AND att.state = 'submitted'
		ORDER BY att.transition_at DESC
		LIMIT 1
	`

	// Unlike repoForTaskSQL, this is not restricted to published tasks: it
	// backs authorization for viewing already-recorded attempts, which must
	// keep resolving even if the task's block is later deleted.
	getTaskCourseIDSQL = `
		SELECT tg.course_id
		FROM tasks t
		JOIN task_groups tg ON tg.id = t.task_group_id
		WHERE t.id = $1
	`
)

// taskCourseSQL is the course of the task $1, for the published view of it.
const taskCourseSQL = `(SELECT tg.course_id FROM tasks t JOIN task_groups tg ON tg.id = t.task_group_id WHERE t.id = $1)`

var (
	// Only a task whose block is published can be attempted.
	repoForTaskSQL = `
		WITH v AS (` + blockView(taskCourseSQL, "NULL") + `)
		SELECT v.course_id, t.task_group_id
		FROM tasks t
		JOIN task_blocks tb ON tb.task_id = t.id
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`

	// The attempt limit is read from the published settings of the task.
	getAttemptLimitSQL = `
		WITH v AS (` + blockView(taskCourseSQL, "NULL") + `)
		SELECT (v.data->>'maxAttempts')::integer AS max_attempts,
		       (SELECT count(*) FROM attempts a
		        WHERE a.task_id = tb.task_id AND a.user_id = $2) AS used
		FROM task_blocks tb
		JOIN v ON v.id = tb.block_id AND NOT v.deleted
		WHERE tb.task_id = $1
	`
)

func (r *PGRepo) AddSSHKey(model *sshkeys.SSHKey) error {
	zap.L().Debug("Executing query", zap.String("query", addSSHKeySQL))

	if _, err := r.db.NamedExec(addSSHKeySQL, model); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return sshkeys.ErrKeyAlreadyRegistered
		}
		return err
	}

	return nil
}

func (r *PGRepo) DeleteSSHKey(ownerID uuid.UUID, fingerprint string) error {
	zap.L().Debug("Executing query", zap.String("query", deleteSSHKeySQL))

	res, err := r.db.Exec(deleteSSHKeySQL, ownerID, fingerprint)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *PGRepo) GetParticipant(fingerprint string) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getParticipantIDSQL))

	var ownerID uuid.UUID

	err := r.db.QueryRow(getParticipantIDSQL, fingerprint).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("ssh key %q not found", fingerprint)
		}
		return uuid.Nil, err
	}

	return ownerID, nil
}

func (r *PGRepo) SaveAttempt(
	repoID pkggit.RepoID,
	taskID uuid.UUID,
	commitHash string,
) error {
	transitionData, err := json.Marshal(struct {
		CommitHash string `json:"commit_hash"`
	}{CommitHash: commitHash})
	if err != nil {
		return fmt.Errorf("marshal transition data: %w", err)
	}
	_, err = r.db.Exec(
		saveAttemptSQL,
		repoID.ParticipantID,
		taskID,
		time.Now(),
		string(transitionData),
	)
	if err != nil {
		return fmt.Errorf("save attempt: %w", err)
	}
	return nil
}

func (r *PGRepo) GetAttemptCommitInfo(
	attemptID uuid.UUID,
) (attempt.AttemptCommitInfo, error) {
	var (
		info           attempt.AttemptCommitInfo
		transitionData json.RawMessage
	)

	err := r.db.QueryRow(getAttemptCommitInfoSQL, attemptID).Scan(
		&info.UserID, &info.TaskID, &transitionData, &info.TaskGroupID, &info.CourseID,
	)
	if err != nil {
		return info, fmt.Errorf(
			"get attempt %s commit info: %w",
			attemptID,
			err,
		)
	}

	var data struct {
		CommitHash string `json:"commit_hash"`
	}
	if err := json.Unmarshal(transitionData, &data); err != nil {
		return info, fmt.Errorf("parse transition_data: %w", err)
	}
	if data.CommitHash == "" {
		return info, fmt.Errorf(
			"attempt %s has no commit_hash in transition_data",
			attemptID,
		)
	}

	info.CommitHash = data.CommitHash
	return info, nil
}

// RepoForPath resolves a course and its live task group by name and the
// owner of the SSH key with fingerprint to their repository.
func (r *PGRepo) RepoForPath(
	ctx context.Context,
	course, group, fingerprint string,
) (pkggit.RepoID, error) {
	zap.L().Debug("Executing query", zap.String("query", repoForPathSQL))

	var id pkggit.RepoID
	err := r.db.QueryRowContext(ctx, repoForPathSQL, course, group, fingerprint).
		Scan(&id.CourseID, &id.TaskGroupID, &id.ParticipantID)
	if errors.Is(err, sql.ErrNoRows) {
		return pkggit.RepoID{}, fmt.Errorf("%w: %s/%s", pkggit.ErrInvalidRepo, course, group)
	}
	if err != nil {
		return pkggit.RepoID{}, fmt.Errorf("resolve repository %s/%s: %w", course, group, err)
	}
	return id, nil
}

func (r *PGRepo) RepoForTask(
	ctx context.Context,
	taskID, participantID uuid.UUID,
) (pkggit.RepoID, error) {
	zap.L().Debug("Executing query", zap.String("query", repoForTaskSQL))

	id := pkggit.RepoID{ParticipantID: participantID}
	err := r.db.QueryRowContext(ctx, repoForTaskSQL, taskID).
		Scan(&id.CourseID, &id.TaskGroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return pkggit.RepoID{}, fmt.Errorf("task %s not found", taskID)
		}
		return pkggit.RepoID{}, fmt.Errorf(
			"resolve repository for task %s: %w",
			taskID,
			err,
		)
	}

	return id, nil
}

// IsCourseMember delegates to the single membership implementation shared
// with course-editing (GetMember) so attempts and course-editing can never
// disagree on who counts as an active course member.
func (r *PGRepo) IsCourseMember(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (bool, error) {
	member, err := r.GetMember(ctx, userID, courseID)
	if err != nil {
		return false, fmt.Errorf("check course membership: %w", err)
	}
	return member != nil && member.IsActive, nil
}

// IsCourseTeacher reports whether userID is an active teacher of courseID,
// used to authorize viewing another participant's attempts.
func (r *PGRepo) IsCourseTeacher(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (bool, error) {
	member, err := r.GetMember(ctx, userID, courseID)
	if err != nil {
		return false, fmt.Errorf("check course teacher: %w", err)
	}
	return member != nil && member.IsActive &&
		member.Role == membership.TeacherRole, nil
}

// GetTaskCourseID resolves the course a task belongs to, for authorization
// purposes only - deliberately not restricted to published tasks.
func (r *PGRepo) GetTaskCourseID(
	ctx context.Context,
	taskID uuid.UUID,
) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskCourseIDSQL))

	var courseID uuid.UUID
	err := r.db.GetContext(ctx, &courseID, getTaskCourseIDSQL, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task %s not found", taskID)
		}
		return uuid.Nil, fmt.Errorf("get task course: %w", err)
	}
	return courseID, nil
}

func (r *PGRepo) GetAttemptLimit(
	ctx context.Context,
	taskID, participantID uuid.UUID,
) (*int, int, error) {
	var row struct {
		Limit *int `db:"max_attempts"`
		Used  int  `db:"used"`
	}
	if err := r.db.GetContext(ctx, &row, getAttemptLimitSQL, taskID, participantID); err != nil {
		return nil, 0, fmt.Errorf("get attempt limit: %w", err)
	}
	return row.Limit, row.Used, nil
}

func (r *PGRepo) CountAttempts(
	ctx context.Context,
	taskID, participantID uuid.UUID,
) (int, error) {
	var used int
	err := r.db.GetContext(
		ctx,
		&used,
		`SELECT count(*) FROM attempts WHERE task_id = $1 AND user_id = $2`,
		taskID,
		participantID,
	)
	if err != nil {
		return 0, fmt.Errorf("count attempts: %w", err)
	}
	return used, nil
}
