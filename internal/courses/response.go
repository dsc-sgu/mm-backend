package courses

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

// UserSummary identifies a user for response enrichment
type UserSummary struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Patronymic string    `json:"patronymic,omitempty"`
	Username   string    `json:"username"`
}

// resolveUserSummary looks up a user's identity for response enrichment
func resolveUserSummary(
	ctx context.Context,
	userService *users.Service,
	userID uuid.UUID,
) *UserSummary {
	user, err := userService.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil
	}
	return &UserSummary{
		ID:         user.ID,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		Patronymic: user.Patronymic,
		Username:   user.Username,
	}
}

// userSummaryResolver memoizes user lookups within a single request so the
// same person (e.g. a teacher who created several snapshots or invites) is
// only fetched once even if they appear multiple times in a list response.
type userSummaryResolver struct {
	userService *users.Service
	cache       map[uuid.UUID]*UserSummary
}

func newUserSummaryResolver(userService *users.Service) *userSummaryResolver {
	return &userSummaryResolver{
		userService: userService,
		cache:       make(map[uuid.UUID]*UserSummary),
	}
}

func (r *userSummaryResolver) resolve(
	ctx context.Context,
	userID uuid.UUID,
) *UserSummary {
	if summary, ok := r.cache[userID]; ok {
		return summary
	}
	summary := resolveUserSummary(ctx, r.userService, userID)
	r.cache[userID] = summary
	return summary
}

type LockConflictBody struct {
	status int
	Detail string       `json:"detail"`
	Holder *UserSummary `json:"holder"`
}

func (e *LockConflictBody) Error() string { return e.Detail }

func (e *LockConflictBody) GetStatus() int { return e.status }

// lockConflictError builds the enriched 423 response
// for a lock held by another user
func (h *Handler) lockConflictError(
	ctx context.Context,
	holderID uuid.UUID,
	baseErr error,
) error {
	holder := resolveUserSummary(ctx, h.userService, holderID)
	if holder == nil {
		return huma.Error423Locked(baseErr.Error())
	}

	return &LockConflictBody{
		status: http.StatusLocked,
		Detail: baseErr.Error(),
		Holder: holder,
	}
}

// CourseIDResponse is the handler-level response containing only a course ID.
type CourseIDResponse struct {
	ID uuid.UUID `json:"id"`
}

// UserRoleResponse is the handler-level response for user role in a course.
type UserRoleResponse struct {
	Role membership.Role `json:"role"`
}

// CourseContentResponse is the handler-level response for course with ordered blocks
type CourseContentResponse struct {
	ID               uuid.UUID       `json:"id"`
	DisciplineID     uuid.UUID       `json:"disciplineId"`
	ActiveSnapshotID *uuid.UUID      `json:"activeSnapshotId"`
	Name             string          `json:"name"`
	Owner            *UserSummary    `json:"owner"`
	CreatedAt        time.Time       `json:"createdAt"`
	Blocks           []*blocks.Block `json:"blocks"`
}

// SnapshotMetadataResponse is a part of response for course timeline
type SnapshotMetadataResponse struct {
	ID        uuid.UUID        `json:"id"`
	CourseID  uuid.UUID        `json:"courseId"`
	Version   int              `json:"version"`
	Status    snapshots.Status `json:"status"`
	CreatedBy *UserSummary     `json:"createdBy"`
	CreatedAt time.Time        `json:"createdAt"`
}

// InviteResponse is the handler-level response for an invite, with the
// creator's identity resolved.
type InviteResponse struct {
	ID           uuid.UUID       `json:"id"`
	CourseID     uuid.UUID       `json:"courseId"`
	ProvidedRole membership.Role `json:"providedRole"`
	CreatedBy    *UserSummary    `json:"createdBy"`
	CreatedAt    time.Time       `json:"createdAt"`
	ExpiresAt    *time.Time      `json:"expiresAt"`
	IsRevoked    bool            `json:"isRevoked"`
}

// InviteDetailsResponse is the handler-level response for an invite's
// details, with the creator's identity resolved.
type InviteDetailsResponse struct {
	ID           uuid.UUID       `json:"id"`
	CourseID     uuid.UUID       `json:"courseId"`
	CourseName   string          `json:"courseName"`
	ProvidedRole membership.Role `json:"providedRole"`
	CreatedBy    *UserSummary    `json:"createdBy"`
	CreatedAt    time.Time       `json:"createdAt"`
	ExpiresAt    *time.Time      `json:"expiresAt"`
	IsRevoked    bool            `json:"isRevoked"`
}

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, membership.ErrPermissionDenied),
		errors.Is(err, membership.ErrNotFound):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrCourseNotFound),
		errors.Is(err, membership.ErrInviteNotFound),
		errors.Is(err, ErrSnapshotNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	case errors.Is(err, ErrSnapshotConflict),
		errors.Is(err, membership.ErrAlreadyMember):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, ErrInvalidTarget):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, membership.ErrInviteRevoked),
		errors.Is(err, membership.ErrInviteExpired):
		return huma.Error410Gone(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}
