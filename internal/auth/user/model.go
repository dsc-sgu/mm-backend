package user

import (
	"time"

	"github.com/google/uuid"
)

type Model struct {
	Id           uuid.UUID `db:"id"`
	CreatedAt    time.Time `db:"created_at"`
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Patronymic   string    `db:"patronymic"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	AvatarUrl    string    `db:"avatar_url"`
	Role         string    `db:"role"`
	PasswordHash []byte    `db:"password_hash"`
	PasswordSalt []byte    `db:"password_salt"`
}

type OutModel struct {
	Id         uuid.UUID `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Patronymic string    `json:"patronymic"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	AvatarUrl  string    `json:"avatarUrl"`
	Role       string    `json:"role"`
}

type CreateModel struct {
	FirstName  string `json:"firstName" binding:"required"`
	LastName   string `json:"lastName" binding:"required"`
	Patronymic string `json:"patronymic" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required"`
	Role       string `json:"role" binding:"required"`
	Password   string `json:"password" binding:"password"`
}
