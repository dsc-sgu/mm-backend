package courses

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type Handler struct {
	courseService *Service
	lockService   *locks.Service
	usersService  *users.Service
}

func NewHandler(
	courseService *Service,
	lockService *locks.Service,
	usersService *users.Service,
) *Handler {
	return &Handler{
		courseService: courseService,
		lockService:   lockService,
		usersService:  usersService,
	}
}

// LockHolderInfo identifies the user who currently holds a course's edit lock.
type LockHolderInfo struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Patronymic string    `json:"patronymic,omitempty"`
	Username   string    `json:"username"`
}

type LockConflictBody struct {
	status int
	Detail string          `json:"detail"`
	Holder *LockHolderInfo `json:"holder"`
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
	holder, err := h.usersService.GetUserByID(ctx, holderID)
	if err != nil || holder == nil {
		return huma.Error423Locked(baseErr.Error())
	}

	return &LockConflictBody{
		status: http.StatusLocked,
		Detail: baseErr.Error(),
		Holder: &LockHolderInfo{
			ID:         holder.ID,
			FirstName:  holder.FirstName,
			LastName:   holder.LastName,
			Patronymic: holder.Patronymic,
			Username:   holder.Username,
		},
	}
}

// CourseIDResponse is the handler-level response containing only a course ID.
type CourseIDResponse struct {
	ID uuid.UUID `json:"id"`
}

// UserRoleResponse is the handler-level response for user role in a course.
type UserRoleResponse struct {
	Role CourseMemberRole `json:"role"`
}

// CourseContentResponse is the handler-level response for course with ordered blocks
type CourseContentResponse struct {
	ID               uuid.UUID       `json:"id"`
	DisciplineID     uuid.UUID       `json:"disciplineId"`
	ActiveSnapshotID *uuid.UUID      `json:"activeSnapshotId"`
	OwnerID          uuid.UUID       `json:"ownerId"`
	Name             string          `json:"name"`
	CreatedAt        time.Time       `json:"createdAt"`
	Blocks           []*blocks.Block `json:"blocks"`
}

// SnapshotMetadataResponse is a part of response for course timeline
type SnapshotMetadataResponse struct {
	ID        uuid.UUID        `json:"id"`
	CourseID  uuid.UUID        `json:"courseId"`
	Version   int              `json:"version"`
	Status    snapshots.Status `json:"status"`
	CreatedBy uuid.UUID        `json:"createdBy"`
	CreatedAt time.Time        `json:"createdAt"`
}

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrCourseMemberNotFound):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, ErrCourseNotFound),
		errors.Is(err, ErrInviteNotFound),
		errors.Is(err, ErrSnapshotNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	case errors.Is(err, ErrSnapshotConflict), errors.Is(err, ErrAlreadyMember):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, ErrInvalidTarget):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, ErrInviteRevoked), errors.Is(err, ErrInviteExpired):
		return huma.Error410Gone(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type CreateCourseInput struct {
	Body CreateCourse
}

type CreateCourseOutput struct {
	Body *CourseIDResponse
}

func (h *Handler) CreateCourse(
	ctx context.Context,
	input *CreateCourseInput,
) (*CreateCourseOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	course, err := h.courseService.CreateCourse(ctx, &input.Body, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &CreateCourseOutput{Body: &CourseIDResponse{ID: course.ID}}, nil
}

type GetPaginatedCoursesInput struct {
	Limit  int    `query:"limit"`
	LastID string `query:"last_id"`
}

type GetPaginatedCoursesOutput struct {
	Body []Course
}

func (h *Handler) GetPaginatedCourses(
	ctx context.Context,
	input *GetPaginatedCoursesInput,
) (*GetPaginatedCoursesOutput, error) {
	var lastID uuid.UUID
	if input.LastID != "" {
		var err error
		lastID, err = uuid.Parse(input.LastID)
		if err != nil {
			return nil, huma.Error400BadRequest("")
		}
	}

	courseList, err := h.courseService.GetPaginatedCourses(
		ctx,
		input.Limit,
		lastID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetPaginatedCoursesOutput{Body: courseList}, nil
}

type GetCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCourseOutput struct {
	Body *Course
}

func (h *Handler) GetCourse(
	ctx context.Context,
	input *GetCourseInput,
) (*GetCourseOutput, error) {
	course, err := h.courseService.GetCourseByID(ctx, input.CourseID)
	if err != nil {
		return nil, handleServiceError(err)
	}
	if course == nil {
		return nil, huma.Error404NotFound("")
	}

	return &GetCourseOutput{Body: course}, nil
}

type GetCourseContentInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCourseContentOutput struct {
	Body *CourseContentResponse
}

func (h *Handler) GetCourseContent(
	ctx context.Context,
	input *GetCourseContentInput,
) (*GetCourseContentOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	content, err := h.courseService.GetCourseContent(
		ctx,
		input.CourseID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetCourseContentOutput{Body: content}, nil
}

type GetSnapshotBlocksInput struct {
	SnapshotID uuid.UUID `path:"snapshot_id"`
}

type GetSnapshotBlocksOutput struct {
	Body []*blocks.Block
}

// GetSnapshotBlocks returns all blocks in a specific snapshot
func (h *Handler) GetSnapshotBlocks(
	ctx context.Context,
	input *GetSnapshotBlocksInput,
) (*GetSnapshotBlocksOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	linkedBlocks, err := h.courseService.GetSnapshotBlocks(
		ctx,
		input.SnapshotID,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetSnapshotBlocksOutput{Body: linkedBlocks}, nil
}

type GetCourseSnapshotsInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCourseSnapshotsOutput struct {
	Body []SnapshotMetadataResponse
}

func (h *Handler) GetCourseSnapshots(
	ctx context.Context,
	input *GetCourseSnapshotsInput,
) (*GetCourseSnapshotsOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	list, err := h.courseService.GetPublishedSnapshots(
		ctx,
		input.CourseID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	result := make([]SnapshotMetadataResponse, len(list))
	for i, s := range list {
		result[i] = SnapshotMetadataResponse{
			ID:        s.ID,
			CourseID:  s.CourseID,
			Version:   s.Version,
			Status:    s.Status,
			CreatedBy: s.CreatedBy,
			CreatedAt: s.CreatedAt,
		}
	}

	return &GetCourseSnapshotsOutput{Body: result}, nil
}

type PatchCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     UpdateCourse
}

type PatchCourseOutput struct {
	Body *Course
}

func (h *Handler) PatchCourse(
	ctx context.Context,
	input *PatchCourseInput,
) (*PatchCourseOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	course, err := h.courseService.UpdateCourseByID(
		ctx,
		input.CourseID,
		userID,
		&input.Body,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}
	return &PatchCourseOutput{Body: course}, nil
}

type DeleteCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

func (h *Handler) DeleteCourse(
	ctx context.Context,
	input *DeleteCourseInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	err := h.courseService.DeleteCourseByID(ctx, input.CourseID, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type LockCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type LockCourseOutput struct {
	Body *InitLockResult
}

func (h *Handler) LockCourse(
	ctx context.Context,
	input *LockCourseInput,
) (*LockCourseOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	lockSession := &locks.LockSession{
		CourseID:  input.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	result, holderID, err := h.courseService.LockAndInitDraft(ctx, lockSession)
	if err != nil {
		if holderID != nil {
			return nil, h.lockConflictError(ctx, *holderID, err)
		}
		return nil, handleServiceError(err)
	}

	return &LockCourseOutput{Body: result}, nil
}

type HeartbeatInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

func (h *Handler) Heartbeat(
	ctx context.Context,
	input *HeartbeatInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	lockSession := &locks.LockSession{
		CourseID:  input.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	holderID, err := h.lockService.RefreshLock(ctx, lockSession)
	if err != nil {
		if holderID != nil {
			return nil, h.lockConflictError(ctx, *holderID, err)
		}
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type SwitchSnapshotInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     struct {
		TargetSnapshotID uuid.UUID `json:"targetSnapshotID"`
	}
}

func (h *Handler) SwitchSnapshot(
	ctx context.Context,
	input *SwitchSnapshotInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	lockSession := &locks.LockSession{
		CourseID:  input.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	err := h.courseService.SwitchSnapshot(
		ctx,
		lockSession,
		input.Body.TargetSnapshotID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type PublishDraftInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     struct {
		DraftSnapshotID uuid.UUID `json:"draftSnapshotID"`
	}
}

func (h *Handler) PublishDraft(
	ctx context.Context,
	input *PublishDraftInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	lockSession := &locks.LockSession{
		CourseID:  input.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	err := h.courseService.PublishDraft(
		ctx,
		lockSession,
		input.Body.DraftSnapshotID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type CancelEditInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

func (h *Handler) CancelEdit(
	ctx context.Context,
	input *CancelEditInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	lockSession := &locks.LockSession{
		CourseID:  input.CourseID,
		UserID:    userID,
		SessionID: sessionID,
	}

	err := h.courseService.CancelEdit(ctx, lockSession)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type CreateInviteInput struct {
	Body CreateInvite
}

type CreateInviteOutput struct {
	Body *Invite
}

func (h *Handler) CreateInvite(
	ctx context.Context,
	input *CreateInviteInput,
) (*CreateInviteOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	invite, err := h.courseService.CreateInvite(ctx, &input.Body, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &CreateInviteOutput{Body: invite}, nil
}

type GetInviteDetailsInput struct {
	InviteID uuid.UUID `path:"invite_id"`
}

type GetInviteDetailsOutput struct {
	Body *InviteDetails
}

func (h *Handler) GetInviteDetails(
	ctx context.Context,
	input *GetInviteDetailsInput,
) (*GetInviteDetailsOutput, error) {
	details, err := h.courseService.GetInviteDetails(ctx, input.InviteID)
	if err != nil {
		return nil, handleServiceError(err)
	}
	return &GetInviteDetailsOutput{Body: details}, nil
}

type JoinCourseByInviteInput struct {
	InviteID uuid.UUID `path:"invite_id"`
}

type JoinCourseByInviteOutput struct {
	Body *CourseIDResponse
}

func (h *Handler) JoinCourseByInvite(
	ctx context.Context,
	input *JoinCourseByInviteInput,
) (*JoinCourseByInviteOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	courseID, err := h.courseService.JoinCourseByInvite(
		ctx,
		input.InviteID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &JoinCourseByInviteOutput{Body: &CourseIDResponse{ID: courseID}}, nil
}

type GetUserRoleInCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetUserRoleInCourseOutput struct {
	Body *UserRoleResponse
}

func (h *Handler) GetUserRoleInCourse(
	ctx context.Context,
	input *GetUserRoleInCourseInput,
) (*GetUserRoleInCourseOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	courseMember, err := h.courseService.CheckCourseMember(
		ctx,
		userID,
		input.CourseID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetUserRoleInCourseOutput{
		Body: &UserRoleResponse{Role: courseMember.Role},
	}, nil
}
