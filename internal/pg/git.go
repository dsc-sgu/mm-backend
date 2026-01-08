package pg

import (
	"database/sql"

	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
