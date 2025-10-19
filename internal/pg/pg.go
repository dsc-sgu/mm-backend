package pg

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/users"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/jmoiron/sqlx"
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
