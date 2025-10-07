package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/MergeMinds/mm-backend-go/internal/auth/password"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
)

var _ user.Repo = (*PGRepo)(nil)

const (
	createUserSql = `
		INSERT INTO users (first_name, last_name, username, email, role, password_hash, password_salt, created_at)
		VALUES (:first_name, :last_name, :username, :email, :role, :password_hash, :password_salt, :created_at)
		RETURNING id, created_at
	`

	getUserByIdSql = `
		SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
		FROM users
		WHERE id = $1
	`

	getUserByEmailSql = `
		SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
		FROM users
		WHERE email = $1
	`

	deleteUserByIdSql = `
		DELETE FROM users
		WHERE id = $1
	`
)

func (r *PGRepo) CreateUser(u *user.CreateModel) (*user.Model, error) {
	passwordSalt, err := password.GenerateSalt()
	if err != nil {
		return nil, nil
	}

	passwordHash := password.Hash(u.Password, passwordSalt)
	zap.L().Debug("Executing query", zap.String("query", createUserSql))

	newUser := user.Model{
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Username:     u.Username,
		Email:        u.Email,
		Role:         u.Role,
		PasswordHash: passwordHash,
		PasswordSalt: passwordSalt,
		CreatedAt:    time.Now(),
	}

	rows, err := r.db.NamedQuery(createUserSql, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newUser.Id, &newUser.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return &newUser, nil
}

func (r *PGRepo) GetUserById(
	ctx context.Context,
	id uuid.UUID,
) (*user.Model, error) {
	zap.L().Debug("Executing query", zap.String("query", getByIdSql))

	var u user.Model
	err := r.db.GetContext(ctx, &u, getByIdSql, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *PGRepo) GetUserByEmail(
	ctx context.Context,
	email string,
) (*user.Model, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByEmailSql))

	var u user.Model
	err := r.db.GetContext(ctx, &u, getUserByEmailSql, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &u, nil
}

func (r *PGRepo) DeleteUserById(id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteUserByIdSql))
	_, err := r.db.ExecContext(context.Background(), deleteUserByIdSql, id)
	return err
}
