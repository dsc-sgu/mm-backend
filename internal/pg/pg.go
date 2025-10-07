package pg

import (
	"github.com/jmoiron/sqlx"
)

type PGRepo struct {
	db *sqlx.DB
}

func NewPGRepo(db *sqlx.DB) Repo {
	return &PGRepo{db}
}
