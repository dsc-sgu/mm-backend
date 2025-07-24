package services

import (
	"context"

	"github.com/MergeMinds/mm-backend-go/internal/repository"
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/google/uuid"
)

type AttemptService interface {
	CreateAttempt(ctx context.Context, req dto.CreateAttempt) (*models.Attempt, error)
	GetAttempt(ctx context.Context, attemptID uuid.UUID) (*models.Attempt, error)
	PatchAttempt(ctx context.Context, attempt *dto.CreateAttempt) (*dto.AssignmentAttempt, error)
	DeleteAttempt(attemptID uuid.UUID) error

	// ListAttemptsForStudentId(ctx context.Context, taskID, userID uuid.UUID) ([]models.Attempt, error)
	// ReviewAttempt(ctx context.Context, req dto.ReviewAttemptRequest) (*models.Attempt, error)
}

type attemptService struct {
	repo repository.AttemptRepository
}

func NewAttemptService(rep repository.AttemptRepository) AttemptService {
	return &attemptService{
		repo: rep,
	}
}

func (a *attemptService) CreateAttempt(ctx context.Context, req dto.CreateAttempt) (*models.Attempt, error) {
	return &dto.AssignmentAttempt{}, nil
}

func (a *attemptService) GetAttempt(ctx context.Context, attemptID uuid.UUID) (*models.Attempt, error) {
	return &dto.AssignmentAttempt{}, nil
}

func (a *attemptService) PatchAttempt(ctx context.Context, attempt *dto.CreateAttempt) (*dto.AssignmentAttempt, error) {
	return &dto.AssignmentAttempt{}, nil
}

func (a *attemptService) DeleteAttempt(id uuid.UUID) error {
	return nil
}
