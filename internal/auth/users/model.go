package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `db:"id"`
	CreatedAt    time.Time `db:"created_at"`
	FirstName    string    `db:"first_name"`
	LastName     string    `db:"last_name"`
	Patronymic   string    `db:"patronymic"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	AvatarURL    string    `db:"avatar_url"`
	Role         string    `db:"role"`
	PasswordHash []byte    `db:"password_hash"`
	PasswordSalt []byte    `db:"password_salt"`
}

type CreateUser struct {
	FirstName  string `json:"firstName"  binding:"required"`
	LastName   string `json:"lastName"   binding:"required"`
	Patronymic string `json:"patronymic" binding:"required"`
	Username   string `json:"username"   binding:"required"`
	Email      string `json:"email"      binding:"required"`
	Role       string `json:"role"       binding:"required"`
	Password   string `json:"password"   binding:"password"`
}

type LoginUser struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterUser struct {
	FirstName  string `json:"firstName"  binding:"required"`
	LastName   string `json:"lastName"   binding:"required"`
	Patronymic string `json:"patronymic" binding:"required"`
	Username   string `json:"username"   binding:"required"`
	Email      string `json:"email"      binding:"required"`
	Password   string `json:"password"   binding:"required"`
}

type LoginResponse struct {
	SessionID uuid.UUID `json:"sessionID"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserID    uuid.UUID `json:"userID"`
}

type RegisterResponse struct {
	ID uuid.UUID `json:"id"`
}
