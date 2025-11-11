package courses

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

func (s *Service) CreateCourse(
	model *CreateCourse,
	ownerId uuid.UUID,
) (*Course, error) {
	return s.repo.CreateCourse(model, ownerId)
}

func (s *Service) GetPaginatedCourses(
	limit int,
	offset int,
) ([]*Course, error) {
	return s.repo.GetPaginatedCourses(limit, offset)
}

func (s *Service) GetCourse(
	ctx context.Context,
	id int,
) (*Course, error) {
	return s.repo.GetCourseById(ctx, id)
}

func (s *Service) PatchCourse(
	id int,
	update *UpdateCourse,
) (*Course, error) {
	return s.repo.UpdateCourseById(id, update)
}

func (s *Service) DeleteCourse(
	id int,
) error {
	return s.repo.DeleteCourseById(id)
}
