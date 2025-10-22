package pg

import (
	"github.com/jmoiron/sqlx"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

type PGRepo struct {
	db *sqlx.DB
}

func NewPGRepo(db *sqlx.DB) *PGRepo {
	return &PGRepo{db}
}

// Check interfaces
var _ users.Repo = (*PGRepo)(nil)
var _ blocks.Repo = (*PGRepo)(nil)
var _ courses.Repo = (*PGRepo)(nil)
var _ disciplines.Repo = (*PGRepo)(nil)
