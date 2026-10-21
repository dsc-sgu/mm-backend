package courses

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

type Handler struct {
	courseService     *Service
	membershipService *membership.Service
	userService       *users.Service
}

func NewHandler(
	courseService *Service,
	membershipService *membership.Service,
	userService *users.Service,
) *Handler {
	return &Handler{
		courseService:     courseService,
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
	// LastName is the name of the course LastID belongs to (the last course
	// of the previous page) - required alongside LastID for pages after the
	// first, since results are keyset-paginated on (name, id) to match the
	// name-ordered listing.
	LastName     string `query:"last_name"`
	DisciplineID string `query:"discipline_id"`
	IsTeacher    bool   `query:"is_teacher"`
	IsStudent    bool   `query:"is_student"`
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
		var err error
		disciplineID, err = uuid.Parse(input.DisciplineID)
		if err != nil {
			return nil, huma.Error400BadRequest("")
		}
	}

	courseList, err := h.courseService.GetPaginatedCourses(
		ctx,
		input.Limit,
		lastID,
		input.LastName,
		CourseFilter{
			DisciplineID: disciplineID,
			UserID:       session.UserIDFromContext(ctx),
			IsTeacher:    input.IsTeacher,
			IsStudent:    input.IsStudent,
		},
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
	// Draft asks for the caller's own draft versions over the published
	// blocks; teachers only.
	Draft bool `query:"draft"`
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
		input.Draft,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetCourseContentOutput{Body: content}, nil
}

type GetCheckpointsInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCheckpointsOutput struct {
	Body []CheckpointResponse
}

// GetCheckpoints lists the publishes of a course, latest first
func (h *Handler) GetCheckpoints(
	ctx context.Context,
	input *GetCheckpointsInput,
) (*GetCheckpointsOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	list, err := h.courseService.GetCheckpoints(ctx, input.CourseID, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	resolver := newUserSummaryResolver(h.userService)
	result := make([]CheckpointResponse, len(list))
	for i, c := range list {
		result[i] = CheckpointResponse{
			ID:        c.ID,
			CourseID:  c.CourseID,
			CreatedBy: resolver.resolve(ctx, c.CreatedBy),
			CreatedAt: c.CreatedAt,
		}
	}

	return &GetCheckpointsOutput{Body: result}, nil
}

type GetCheckpointBlocksInput struct {
	CourseID     uuid.UUID `path:"course_id"`
	CheckpointID uuid.UUID `path:"checkpoint_id"`
}

type GetCheckpointBlocksOutput struct {
	Body []*blocks.Block
}

// GetCheckpointBlocks returns the course's blocks as a publish left them
func (h *Handler) GetCheckpointBlocks(
	ctx context.Context,
	input *GetCheckpointBlocksInput,
) (*GetCheckpointBlocksOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	linkedBlocks, err := h.courseService.GetCheckpointBlocks(
		ctx,
		input.CourseID,
		input.CheckpointID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetCheckpointBlocksOutput{Body: linkedBlocks}, nil
}

type RestoreCheckpointInput struct {
	CourseID     uuid.UUID `path:"course_id"`
	CheckpointID uuid.UUID `path:"checkpoint_id"`
}

type RestoreCheckpointOutput struct {
	Body *CheckpointResponse
}

// RestoreCheckpoint publishes the course as it was at the checkpoint
func (h *Handler) RestoreCheckpoint(
	ctx context.Context,
	input *RestoreCheckpointInput,
) (*RestoreCheckpointOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	c, err := h.courseService.RestoreCheckpoint(
		ctx,
		input.CourseID,
		input.CheckpointID,
		userID,
	)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &RestoreCheckpointOutput{Body: &CheckpointResponse{
		ID:        c.ID,
		CourseID:  c.CourseID,
		CreatedBy: resolveUserSummary(ctx, h.userService, c.CreatedBy),
		CreatedAt: c.CreatedAt,
	}}, nil
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

type PublishInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type PublishOutput struct {
	Body *CheckpointResponse
}

// Publish publishes the caller's draft blocks of the course
func (h *Handler) Publish(
	ctx context.Context,
	input *PublishInput,
) (*PublishOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	c, err := h.courseService.Publish(ctx, input.CourseID, userID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &PublishOutput{Body: &CheckpointResponse{
		ID:        c.ID,
		CourseID:  c.CourseID,
		CreatedBy: resolveUserSummary(ctx, h.userService, c.CreatedBy),
		CreatedAt: c.CreatedAt,
	}}, nil
}

type CancelEditInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

// CancelEdit discards the caller's draft blocks of the course
func (h *Handler) CancelEdit(
	ctx context.Context,
	input *CancelEditInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	if err := h.courseService.CancelEdit(ctx, input.CourseID, userID); err != nil {
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
