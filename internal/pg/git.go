package pg

import (
	"database/sql"

	"github.com/google/uuid"
	"go.uber.org/zap"

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

func GetTask(name string) (uuid.UUID, error) {
	return uuid.Parse("e7cf6012-1348-434b-9d54-bd89c9e6e95e")
}

func (r *PGRepo) GetParticipant(fingerprint string) (uuid.UUID, error) {
	zap.L().Debug("Executing query", zap.String("query", getParticipantIdSQL))

	var ownerID uuid.UUID

	err := r.db.QueryRow(getParticipantIdSQL, fingerprint).Scan(&ownerID)

	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	return ownerID, nil
}
