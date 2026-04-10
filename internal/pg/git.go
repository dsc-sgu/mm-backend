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

// realise via db
func GetParticipant(fingerprint string) (uuid.UUID, error) {
	if fingerprint == "SHA256:AH71wflD7hbxs0bGhssvTy77dLoszYUWXkeK798ph04" {
		return uuid.Parse("681ae49f-1f56-4632-bd1b-7ca3ab09a467")
	} else {
		return uuid.Parse("e7cf6012-1348-434b-9d54-bd89c9e6e95e")
	}
}
