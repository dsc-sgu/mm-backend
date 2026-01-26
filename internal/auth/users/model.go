package users

import (
	"time"

	"github.com/google/uuid"
)

type Model struct {
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

type OutModel struct {
	ID         uuid.UUID `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Patronymic string    `json:"patronymic"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	AvatarURL  string    `json:"avatarURL"`
	Role       string    `json:"role"`
}

type CreateModel struct {
	FirstName  string `json:"firstName"  binding:"required"`
	LastName   string `json:"lastName"   binding:"required"`
	Patronymic string `json:"patronymic" binding:"required"`
	Username   string `json:"username"   binding:"required"`
	Email      string `json:"email"      binding:"required"`
	Role       string `json:"role"       binding:"required"`
	Password   string `json:"password"   binding:"password"`
}

type LoginModel struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterModel struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName"  binding:"required"`
	Username  string `json:"username"  binding:"required"`
	Email     string `json:"email"     binding:"required"`
	Password  string `json:"password"  binding:"required"`
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
