package snapshots

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateSnapshot(
	ctx context.Context,
	tx *sqlx.Tx,
	snapshot *Snapshot,
) (*Snapshot, error) {
	return s.repo.CreateSnapshot(ctx, tx, snapshot)
}

func (s *Service) GetSnapshotByID(
	ctx context.Context,
	id uuid.UUID,
) (*Snapshot, error) {
	return s.repo.GetSnapshotByID(ctx, id)
}

func (s *Service) GetPublishedSnapshotsByCourseID(
	ctx context.Context,
	courseID uuid.UUID,
) ([]*Snapshot, error) {
	return s.repo.GetPublishedSnapshotsByCourseID(ctx, courseID)
}

func (s *Service) FindUserDraft(
	ctx context.Context,
	courseID, userID uuid.UUID,
) (*Snapshot, error) {
	return s.repo.FindUserDraft(ctx, courseID, userID)
}

func (s *Service) UpdateSnapshotStatus(
	ctx context.Context,
	tx *sqlx.Tx,
	id uuid.UUID,
	status Status,
) error {
	return s.repo.UpdateSnapshotStatus(ctx, tx, id, status)
}

func (s *Service) CreateDraftFromActual(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
	targetVersion int,
	userID uuid.UUID,
	actualSnapshotID uuid.UUID,
) (*Snapshot, error) {
	return s.repo.CreateDraftFromActual(
		ctx,
		tx,
		courseID,
		targetVersion,
		userID,
		actualSnapshotID,
	)
}

func (s *Service) SwitchSnapshotContent(
	ctx context.Context,
	tx *sqlx.Tx,
	draftSnapshotID uuid.UUID,
	targetSnapshotID uuid.UUID,
) error {
	return s.repo.SwitchSnapshotContent(
		ctx,
		tx,
		draftSnapshotID,
		targetSnapshotID,
	)
}

func (s *Service) DeleteAllSnapshotsByCourseID(
	ctx context.Context,
	tx *sqlx.Tx,
	courseID uuid.UUID,
) error {
	return s.repo.DeleteAllSnapshotsByCourseID(ctx, tx, courseID)
}

func (s *Service) DiscardDraft(
	ctx context.Context,
	tx *sqlx.Tx,
	draftSnapshotID uuid.UUID,
) error {
	return s.repo.DiscardDraft(ctx, tx, draftSnapshotID)
}
