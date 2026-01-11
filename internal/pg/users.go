package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/auth/password"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
)

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

func (r *PGRepo) CreateUser(user *users.CreateModel) (*users.Model, error) {
	passwordSalt, err := password.GenerateSalt()
	if err != nil {
		return nil, nil
	}

	passwordHash := password.Hash(user.Password, passwordSalt)
	zap.L().Debug("Executing query", zap.String("query", createUserSql))

	newUser := users.Model{
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
		return nil, fmt.Errorf("create user: insert in db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newUser.Id, &newUser.CreatedAt); err != nil {
			return nil, fmt.Errorf("create user: scan user id: %w", err)
		}
	}

	return &newUser, nil
}

func (r *PGRepo) GetUserById(
	ctx context.Context,
	id uuid.UUID,
) (*users.Model, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByIdSql))

	var u users.Model
	err := r.db.GetContext(ctx, &u, getUserByIdSql, id)
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
) (*users.Model, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByEmailSql))

	var u users.Model
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
