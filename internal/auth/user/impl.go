package user

import (
	"context"
	"database/sql"
	"time"

	"github.com/MergeMinds/mm-backend-go/internal/auth/password"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type PGRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewPGRepo(db *sqlx.DB, logger *zap.Logger) Repo {
	return &PGRepo{db, logger}
}

const createUserSql = `
	INSERT INTO users (first_name, last_name, username, email, role, password_hash, password_salt, created_at)
	VALUES (:first_name, :last_name, :username, :email, :role, :password_hash, :password_salt, :created_at)
	RETURNING id, created_at
`

func (r *PGRepo) Create(user *CreateModel) (*Model, error) {
	passwordSalt, err := password.GenerateSalt()
	if err != nil {
		return nil, nil
	}

	passwordHash := password.Hash(user.Password, passwordSalt)
	r.logger.Debug("Executing query", zap.String("query", createUserSql))

	newUser := Model{
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Username:     user.Username,
		Email:        user.Email,
		Role:         user.Role,
		PasswordHash: passwordHash,
		PasswordSalt: passwordSalt,
		CreatedAt:    time.Now(),
	}

	rows, err := r.db.NamedQuery(createUserSql, newUser)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&newUser.Id, &newUser.CreatedAt); err != nil {
			return nil, err
		}
	}

	return &newUser, nil
}

const getByIdSql = `
	SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
	FROM users
	WHERE id = $1
`

func (r *PGRepo) GetById(id uuid.UUID) (*Model, error) {
	r.logger.Debug("Executing query", zap.String("query", getByIdSql))

	var user Model
	err := r.db.GetContext(context.Background(), &user, getByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

const getByEmailSql = `
	SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
	FROM users
	WHERE email = $1
`

func (r *PGRepo) GetByEmail(email string) (*Model, error) {
	r.logger.Debug("Executing query", zap.String("query", getByEmailSql))

	var user Model
	err := r.db.GetContext(context.Background(), &user, getByEmailSql, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

const deleteByIdSql = `
	DELETE FROM users
	WHERE id = $1
`

func (r *PGRepo) DeleteById(id uuid.UUID) error {
	r.logger.Debug("Executing query", zap.String("query", deleteByIdSql))
	_, err := r.db.ExecContext(context.Background(), deleteByIdSql, id)
	return err
}
