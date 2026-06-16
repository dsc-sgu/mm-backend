package pg

import (
	"github.com/jmoiron/sqlx"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type PGRepo struct {
	db *sqlx.DB
}

func NewPGRepo(db *sqlx.DB) *PGRepo {
	return &PGRepo{db}
}

// Check interfaces
var (
	_ users.Repo       = (*PGRepo)(nil)
	_ blocks.Repo      = (*PGRepo)(nil)
	_ courses.Repo     = (*PGRepo)(nil)
	_ disciplines.Repo = (*PGRepo)(nil)
	_ git.DBRepo       = (*PGRepo)(nil)
	_ tasks.Repo       = (*PGRepo)(nil)
	_ attempt.Repo     = (*PGRepo)(nil)
	_ snapshots.Repo   = (*PGRepo)(nil)
	_ locks.Repo       = (*PGRepo)(nil)
)
