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
	createUserSQL = `
		INSERT INTO users (first_name, last_name, patronymic, username, email, role, password_hash, password_salt, created_at)
		VALUES (:first_name, :last_name, :patronymic, :username, :email, :role, :password_hash, :password_salt, :created_at)
		RETURNING id, created_at
	`

	getUserByIdSQL = `
		SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
		FROM users
		WHERE id = $1
	`

	getUserByUsernameSQL = `
		SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
		FROM users
		WHERE username = $1
	`
	getUserByEmailSQL = `
		SELECT id, first_name, last_name, username, email, role, password_hash, password_salt, created_at
		FROM users
		WHERE email = $1
	`

	deleteUserByIdSQL = `
		DELETE FROM users
		WHERE id = $1
	`
)

func (r *PGRepo) CreateUser(
	ctx context.Context,
	user *users.CreateUser,
) (*users.User, error) {
	passwordSalt, err := password.GenerateSalt()
	if err != nil {
		return nil, nil
	}

	passwordHash := password.Hash(user.Password, passwordSalt)
	zap.L().Debug("Executing query", zap.String("query", createUserSQL))

	newUser := users.User{
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Patronymic:   user.Patronymic,
		Username:     user.Username,
		Email:        user.Email,
		Role:         user.Role,
		PasswordHash: passwordHash,
		PasswordSalt: passwordSalt,
		CreatedAt:    time.Now(),
	}

	rows, err := r.db.NamedQueryContext(ctx, createUserSQL, newUser)
	if err != nil {
		return nil, fmt.Errorf("create user: insert in db: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&newUser.ID, &newUser.CreatedAt); err != nil {
			return nil, fmt.Errorf("create user: scan user id: %w", err)
		}
	}

	return &newUser, nil
}

func (r *PGRepo) GetUserByID(
	ctx context.Context,
	id uuid.UUID,
) (*users.User, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByIdSQL))

	var u users.User
	err := r.db.GetContext(ctx, &u, getUserByIdSQL, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *PGRepo) GetUserByUsername(
	ctx context.Context,
	username string,
) (*users.User, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByUsernameSQL))

	var u users.User
	err := r.db.GetContext(ctx, &u, getUserByUsernameSQL, username)
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
) (*users.User, error) {
	zap.L().Debug("Executing query", zap.String("query", getUserByEmailSQL))

	var u users.User
	err := r.db.GetContext(ctx, &u, getUserByEmailSQL, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &u, nil
}

func (r *PGRepo) DeleteUserByID(ctx context.Context, id uuid.UUID) error {
	zap.L().Debug("Executing query", zap.String("query", deleteUserByIdSQL))
	_, err := r.db.ExecContext(ctx, deleteUserByIdSQL, id)
	return err
}
