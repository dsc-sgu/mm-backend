package repository

import (
	"context"
)

type AttemptRepository interface {
	CreateAttempt(ctx context.Context, attempt *models.Attempt) (*models.Attempt, error)
	GetAttemptByID(ctx context.Context, id int) (*models.Attempt, error)
	UpdateAttempt(ctx context.Context, id int) (*models.Attempt, error)
	DeleteAttempt(ctx context.Context, id int) (*models.Attempt, error)
}

func NewAttemptRepo() AttemptRepository {
	return &attemptRepo{}
}

type attemptRepo struct {
}

func (a *attemptRepo) CreateAttempt(ctx context.Context, attempt *models.Attempt) (*models.Attempt, error) {
	return &models.Attempt{}, nil
}

func (a *attemptRepo) GetAttemptByID(ctx context.Context, id int) (*models.Attempt, error) {
	return &models.Attempt{}, nil
}

func (a *attemptRepo) UpdateAttempt(ctx context.Context, id int) (*models.Attempt, error) {
	return &models.Attempt{}, nil
}

func (a *attemptRepo) DeleteAttempt(ctx context.Context, id int) (*models.Attempt, error) {
	return &models.Attempt{}, nil
}
