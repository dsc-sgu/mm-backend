package blocks

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type Handler struct {
	svc          *Service
	snapshotRepo snapshots.Repo
	lockService  *locks.Service
}

func NewHandler(
	svc *Service,
	snapshotRepo snapshots.Repo,
	lockService *locks.Service,
) *Handler {
	return &Handler{
		svc:          svc,
		snapshotRepo: snapshotRepo,
		lockService:  lockService,
	}
}

// CreateBlockResponse is the handler-level response for block creation.
type CreateBlockResponse struct {
	ID uuid.UUID `json:"id"`
}

type GetBlockInput struct {
	BlockID uuid.UUID `path:"block_id"`
}

type GetBlockOutput struct {
	Body *Block
}

func (h *Handler) GetBlock(
	ctx context.Context,
	input *GetBlockInput,
) (*GetBlockOutput, error) {
	block, err := h.svc.GetBlockByID(ctx, input.BlockID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if block == nil {
		return nil, huma.Error404NotFound("")
	}
	return &GetBlockOutput{Body: block}, nil
}

// validateLock checks if the user has a valid lock on the course.
func (h *Handler) validateLock(
	ctx context.Context,
	snapshotID uuid.UUID,
) error {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil {
		return huma.Error401Unauthorized("")
	}

	// Get the target snapshot for CourseID
	snapshot, err := h.snapshotRepo.GetSnapshotByID(ctx, snapshotID)
	if err != nil || snapshot == nil {
		return huma.Error404NotFound("target snapshot not found")
	}

	if snapshot.Status != snapshots.DraftStatus {
		return huma.Error400BadRequest(
			"cannot modify blocks in a non-draft snapshot",
		)
	}

	lockSession := &locks.LockSession{
		CourseID:  snapshot.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	isValid, err := h.lockService.ValidateLock(ctx, lockSession)
	if err != nil {
		return huma.Error500InternalServerError("failed to verify lock status")
	}
	if !isValid {
		return huma.Error423Locked(
			"mutation rejected: your editing session is invalid or expired",
		)
	}

	return nil
}

type CreateBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	Body       CreateBlock
}

type CreateBlockOutput struct {
	Body *CreateBlockResponse
}

func (h *Handler) CreateBlock(
	ctx context.Context,
	input *CreateBlockInput,
) (*CreateBlockOutput, error) {
	if err := h.validateLock(ctx, input.SnapshotID); err != nil {
		return nil, err
	}

	input.Body.SnapshotID = input.SnapshotID

	block, err := h.svc.CreateBlock(ctx, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create block")
	}

	return &CreateBlockOutput{Body: &CreateBlockResponse{ID: block.ID}}, nil
}

type MoveBlockInput struct {
	SnapshotID   uuid.UUID  `path:"snapshot_id"`
	BlockID      uuid.UUID  `path:"block_id"`
	AfterBlockID *uuid.UUID `                   json:"afterBlockID"`
}

func (h *Handler) MoveBlock(
	ctx context.Context,
	input *MoveBlockInput,
) (*struct{}, error) {
	if err := h.validateLock(ctx, input.SnapshotID); err != nil {
		return nil, err
	}

	err := h.svc.MoveBlock(
		ctx,
		input.BlockID,
		input.SnapshotID,
		input.AfterBlockID,
	)
	if err != nil {
		return nil, huma.Error500InternalServerError(
			"failed to move block: " + err.Error(),
		)
	}

	return nil, nil
}

type PatchBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
	Body       UpdateBlock
}

type PatchBlockOutput struct {
	Body *Block
}

func (h *Handler) PatchBlock(
	ctx context.Context,
	input *PatchBlockInput,
) (*PatchBlockOutput, error) {
	if err := h.validateLock(ctx, input.SnapshotID); err != nil {
		return nil, err
	}

	block, err := h.svc.UpdateBlockContent(ctx, input.BlockID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(
			"failed to update block data",
		)
	}

	return &PatchBlockOutput{Body: block}, nil
}

type DeleteBlockInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
	BlockID    uuid.UUID `path:"block_id"`
}

func (h *Handler) DeleteBlock(
	ctx context.Context,
	input *DeleteBlockInput,
) (*struct{}, error) {
	if err := h.validateLock(ctx, input.SnapshotID); err != nil {
		return nil, err
	}

	if err := h.svc.DeleteBlockByID(ctx, input.BlockID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete block")
	}

	return nil, nil
}
