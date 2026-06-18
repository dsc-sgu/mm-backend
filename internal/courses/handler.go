package courses

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

type Handler struct {
	courseService     *Service
	lockService       *locks.Service
	membershipService *membership.Service
	userService       *users.Service
}

func NewHandler(
	courseService *Service,
	lockService *locks.Service,
	membershipService *membership.Service,
	userService *users.Service,
) *Handler {
	return &Handler{
		courseService:     courseService,
		lockService:       lockService,
		membershipService: membershipService,
		userService:       userService,
	}
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

	var disciplineID uuid.UUID
	if input.DisciplineID != "" {
		disciplineID, err = uuid.Parse(input.DisciplineID)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid discipline_id")
		}
	}

	courseList, err := h.courseService.GetPaginatedCourses(
		ctx,
		input.Limit,
		lastID,
		disciplineID,
		userID,
		teacherBool,
		studentBool,
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
	CourseID   uuid.UUID `path:"course_id"`
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
		input.CourseID,
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

	resolver := newUserSummaryResolver(h.userService)
	result := make([]SnapshotMetadataResponse, len(list))
	for i, s := range list {
		result[i] = SnapshotMetadataResponse{
			ID:        s.ID,
			CourseID:  s.CourseID,
			Version:   s.Version,
			Status:    s.Status,
			CreatedBy: resolver.resolve(ctx, s.CreatedBy),
			CreatedAt: s.CreatedAt,
		}
	}

	return &GetCourseSnapshotsOutput{Body: result}, nil
}

type GetCourseSnapshotInput struct {
	CourseID   uuid.UUID `path:"course_id"`
	SnapshotID uuid.UUID `path:"snapshot_id"`
}

type GetCourseSnapshotOutput struct {
	Body *SnapshotMetadataResponse
}

func (h *Handler) GetCourseSnapshot(
	ctx context.Context,
	input *GetCourseSnapshotInput,
) (*GetCourseSnapshotOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	s, err := h.courseService.GetCourseSnapshot(
		ctx,
		input.SnapshotID,
		input.CourseID,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetCourseSnapshotOutput{
		Body: &SnapshotMetadataResponse{
			ID:        s.ID,
			CourseID:  s.CourseID,
			Version:   s.Version,
			Status:    s.Status,
			CreatedBy: resolveUserSummary(ctx, h.userService, s.CreatedBy),
			CreatedAt: s.CreatedAt,
		},
	}, nil
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
	CourseID uuid.UUID `path:"course_id"`
	Body     membership.CreateInvite
}

type CreateInviteOutput struct {
	Body *InviteResponse
}

func (h *Handler) CreateInvite(
	ctx context.Context,
	input *CreateInviteInput,
) (*CreateInviteOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	input.Body.CourseID = input.CourseID

	invite, err := h.membershipService.CreateInvite(ctx, &input.Body, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &CreateInviteOutput{Body: &InviteResponse{
		ID:           invite.ID,
		CourseID:     invite.CourseID,
		ProvidedRole: invite.ProvidedRole,
		CreatedBy:    resolveUserSummary(ctx, h.userService, invite.CreatedBy),
		CreatedAt:    invite.CreatedAt,
		ExpiresAt:    invite.ExpiresAt,
		IsRevoked:    invite.IsRevoked,
	}}, nil
}

type GetCourseInvitesInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCourseInvitesOutput struct {
	Body []InviteResponse
}

func (h *Handler) GetCourseInvites(
	ctx context.Context,
	input *GetCourseInvitesInput,
) (*GetCourseInvitesOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	inviteList, err := h.membershipService.GetInvitesByCourseID(
		ctx,
		input.CourseID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	resolver := newUserSummaryResolver(h.userService)
	result := make([]InviteResponse, len(inviteList))
	for i, invite := range inviteList {
		result[i] = InviteResponse{
			ID:           invite.ID,
			CourseID:     invite.CourseID,
			ProvidedRole: invite.ProvidedRole,
			CreatedBy:    resolver.resolve(ctx, invite.CreatedBy),
			CreatedAt:    invite.CreatedAt,
			ExpiresAt:    invite.ExpiresAt,
			IsRevoked:    invite.IsRevoked,
		}
	}

	return &GetCourseInvitesOutput{Body: result}, nil
}

type GetInviteDetailsInput struct {
	InviteID uuid.UUID `path:"invite_id"`
}

type GetInviteDetailsOutput struct {
	Body *InviteDetailsResponse
}

func (h *Handler) GetInviteDetails(
	ctx context.Context,
	input *GetInviteDetailsInput,
) (*GetInviteDetailsOutput, error) {
	details, err := h.membershipService.GetInviteDetails(ctx, input.InviteID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetInviteDetailsOutput{Body: &InviteDetailsResponse{
		ID:           details.ID,
		CourseID:     details.CourseID,
		CourseName:   details.CourseName,
		ProvidedRole: details.ProvidedRole,
		CreatedBy: resolveUserSummary(
			ctx,
			h.userService,
			details.CreatedBy,
		),
		CreatedAt: details.CreatedAt,
		ExpiresAt: details.ExpiresAt,
		IsRevoked: details.IsRevoked,
	}}, nil
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

	courseID, err := h.membershipService.JoinCourseByInvite(
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
