package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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
		JOIN tasks t ON a.task_id = t.block_id
		JOIN task_groups tg ON t.task_group_id = tg.id
		WHERE a.id = $1 AND att.state = 'submitted'
		ORDER BY att.transition_at DESC
		LIMIT 1
	`

	repoForTaskSQL = `
		SELECT cs.course_id, t.task_group_id
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id
		JOIN course_snapshots cs ON cs.id = b.snapshot_id
		JOIN courses c ON c.id = cs.course_id
		WHERE t.block_id = $1 AND b.snapshot_id = c.active_snapshot_id
	`

	// Unlike repoForTaskSQL, this is not restricted to the active snapshot:
	// it backs authorization for viewing already-recorded attempts, which
	// must keep resolving even if the task later drops out of the active
	// snapshot.
	getTaskCourseIDSQL = `
		SELECT cs.course_id
		FROM tasks t
		JOIN blocks b ON b.id = t.block_id
		JOIN course_snapshots cs ON cs.id = b.snapshot_id
		WHERE t.block_id = $1
	`
)

func (r *PGRepo) AddSSHKey(model *sshkeys.SSHKey) error {
	zap.L().Debug("Executing query", zap.String("query", addSSHKeySQL))

	if _, err := r.db.NamedExec(addSSHKeySQL, model); err != nil {
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

func (r *PGRepo) SaveAttempt(repoID pkggit.RepoID, taskID uuid.UUID, commitHash string) error {
	transitionData := fmt.Sprintf(`{"commit_hash":"%s"}`, commitHash)
	_, err := r.db.Exec(saveAttemptSQL, repoID.ParticipantID, taskID, time.Now(), transitionData)
	if err != nil {
		return fmt.Errorf("save attempt: %w", err)
	}
	return nil
}

func (r *PGRepo) GetAttemptCommitInfo(attemptID uuid.UUID) (attempt.AttemptCommitInfo, error) {
	var (
		info           attempt.AttemptCommitInfo
		transitionData json.RawMessage
	)

	err := r.db.QueryRow(getAttemptCommitInfoSQL, attemptID).Scan(
		&info.UserID, &info.TaskID, &transitionData, &info.TaskGroupID, &info.CourseID,
	)
	if err != nil {
		return info, fmt.Errorf("get attempt %s commit info: %w", attemptID, err)
	}

	var data struct {
		CommitHash string `json:"commit_hash"`
	}
	if err := json.Unmarshal(transitionData, &data); err != nil {
		return info, fmt.Errorf("parse transition_data: %w", err)
	}
	if data.CommitHash == "" {
		return info, fmt.Errorf("attempt %s has no commit_hash in transition_data", attemptID)
	}

	info.CommitHash = data.CommitHash
	return info, nil
}

func (r *PGRepo) GetCourse(ctx context.Context, name string) (uuid.UUID, error) {
	course, err := r.GetCourseByName(ctx, name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("course %q: %w", name, err)
	}

	return course.ID, nil
}

func (r *PGRepo) RepoForTask(ctx context.Context, taskID, participantID uuid.UUID) (pkggit.RepoID, error) {
	zap.L().Debug("Executing query", zap.String("query", repoForTaskSQL))

	id := pkggit.RepoID{ParticipantID: participantID}
	err := r.db.QueryRowContext(ctx, repoForTaskSQL, taskID).
		Scan(&id.CourseID, &id.TaskGroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return pkggit.RepoID{}, fmt.Errorf("task %s not found", taskID)
		}
		return pkggit.RepoID{}, fmt.Errorf("resolve repository for task %s: %w", taskID, err)
	}

	return id, nil
}

// IsCourseMember delegates to the single membership implementation shared
// with course-editing (GetMember) so attempts and course-editing can never
// disagree on who counts as an active course member.
func (r *PGRepo) IsCourseMember(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	member, err := r.GetMember(ctx, userID, courseID)
	if err != nil {
		return false, fmt.Errorf("check course membership: %w", err)
	}
	return member != nil && member.IsActive, nil
}

// IsCourseTeacher reports whether userID is an active teacher of courseID,
// used to authorize viewing another participant's attempts.
func (r *PGRepo) IsCourseTeacher(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	member, err := r.GetMember(ctx, userID, courseID)
	if err != nil {
		return false, fmt.Errorf("check course teacher: %w", err)
	}
	return member != nil && member.IsActive && member.Role == membership.TeacherRole, nil
}

// GetTaskCourseID resolves the course a task belongs to, for authorization
// purposes only — deliberately not scoped to the active snapshot.
func (r *PGRepo) GetTaskCourseID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error) {
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
