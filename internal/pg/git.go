package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/internal/git"
)

const (
	addSshKeySQL = `
		INSERT INTO ssh_keys (owner_id, name, key, fingerprint, created_at)
		VALUES (:owner_id, :name, :key, :fingerprint, :created_at)
	`

	deleteSshKeySQL = `
		DELETE FROM ssh_keys
		WHERE owner_id = $1 AND fingerprint = $2
	`

	getParticipantIdSQL = `
		SELECT owner_id FROM ssh_keys
		WHERE fingerprint = $1
	`

	getTaskSQL = `
		SELECT t.block_id FROM tasks t
		JOIN blocks b ON b.id = t.block_id
		WHERE b.data->>'name' = $1
	`
	saveAttemptSQL = `
		WITH new_attempt AS (
			INSERT INTO attempts (user_id, task_id)
			VALUES ($1, $2)
			RETURNING id )
		INSERT INTO attempt_transitions (attempt_id, state, transition_at, transition_data)
		VALUES ((SELECT id FROM new_attempt), 'submitted', $3, $4::jsonb)
	`
)

func (r *PGRepo) AddSshKey(model *git.SshKey) error {
	zap.L().Debug("Executing query", zap.String("query", addSshKeySQL))

	if _, err := r.db.NamedExec(addSshKeySQL, model); err != nil {
		return err
	}

	return nil
}

func (r *PGRepo) DeleteSshKey(ownerId uuid.UUID, fingerprint string) error {
	zap.L().Debug("Executing query", zap.String("query", deleteSshKeySQL))

	res, err := r.db.Exec(deleteSshKeySQL, ownerId, fingerprint)
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
	zap.L().Debug("Executing query", zap.String("query", getParticipantIdSQL))

	var ownerID uuid.UUID

	err := r.db.QueryRow(getParticipantIdSQL, fingerprint).Scan(&ownerID)

	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("ssh key %q not found", fingerprint)
		}
		return uuid.Nil, err
	}

	return ownerID, nil
}

func (r *PGRepo) CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool {
	fingerprint := gossh.FingerprintSHA256(pk)
	_, err := r.GetParticipant(fingerprint)
	return err == nil
}

func (r *PGRepo) CheckPasswordAuth(ctx ssh.Context, password string) bool {
	return false
}

func (r *PGRepo) GetTask(name string) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getTaskSQL))

	var taskID uuid.UUID

	err := r.db.QueryRow(getTaskSQL, name).Scan(&taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("task %q not found", name)
		}
		return uuid.Nil, err
	}

	return taskID, nil
}

func (r *PGRepo) GetCourse(name string) (uuid.UUID, error) {
	ctx := context.Background()
	course, err := r.GetCourseByName(ctx, name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("course %q: %w", name, err)
	}

	return course.ID, nil
}

func (r *PGRepo) SaveAttempt(repoID git.RepoID, commitHash string) error {
	transitionData := fmt.Sprintf(`{"commit_hash":"%s"}`, commitHash)
	_, err := r.db.Exec(saveAttemptSQL, repoID.ParticipantID, repoID.TaskID, time.Now(), transitionData)
	if err != nil {
		return fmt.Errorf("save attempt: %w", err)
	}
	return nil
}
