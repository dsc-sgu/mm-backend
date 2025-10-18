package pg

import (
  "github.com/MergeMinds/mm-backend-go/internal/auth/users"
	"github.com/jmoiron/sqlx"
)

type PGRepo struct {
	db *sqlx.DB
}

func NewPGRepo(db *sqlx.DB) *PGRepo {
	return &PGRepo{db}
}

var _ users.Repo = (*PGRepo)(nil)
