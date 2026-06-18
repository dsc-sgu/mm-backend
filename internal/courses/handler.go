package courses

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

// CourseIDResponse is the handler-level response containing only a course ID.
type CourseIDResponse struct {
	ID uuid.UUID `json:"id"`
}

// UserRoleResponse is the handler-level response for user role in a course.
type UserRoleResponse struct {
	Role CourseMemberRole `json:"role"`
}

type Handler struct {
	courseService *Service
	blockService  *blocks.Service
}

func NewHandler(courseService *Service, blockService *blocks.Service) *Handler {
	return &Handler{courseService: courseService, blockService: blockService}
}

type CreateCourseInput struct {
	Body CreateCourse
}

type CreateCourseOutput struct {
	Body *CourseIDResponse
}

func (h *Handler) CreateCourse(ctx context.Context, input *CreateCourseInput) (*CreateCourseOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	course, err := h.courseService.CreateCourse(ctx, &input.Body, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	return &CreateCourseOutput{Body: &CourseIDResponse{ID: course.ID}}, nil
}

type GetPaginatedCoursesInput struct {
	Limit        int    `query:"limit"`
	LastID       string `query:"last_id"`
	DisciplineID string `query:"discipline_id"`
	IsTeacher    string `query:"is_teacher"`
	IsStudent    string `query:"is_student"`
}

type GetPaginatedCoursesOutput struct {
	Body []Course
}

func (h *Handler) GetPaginatedCourses(
	ctx context.Context,
	input *GetPaginatedCoursesInput,
) (*GetPaginatedCoursesOutput, error) {
	var err error
	teacherBool := false
	if input.IsTeacher != "" {
		teacherBool, err = strconv.ParseBool(input.IsTeacher)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid is_teacher")
		}
	}

	studentBool := false
	if input.IsStudent != "" {
		studentBool, err = strconv.ParseBool(input.IsStudent)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid is_student")
		}
	}

	userID := uuid.Nil
	if teacherBool || studentBool {
		userID = session.UserIDFromContext(ctx)
	}

	var lastID uuid.UUID
	if input.LastID != "" {
		lastID, err = uuid.Parse(input.LastID)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid last_id")
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
		return nil, huma.Error500InternalServerError("")
	}

	return &GetPaginatedCoursesOutput{Body: courseList}, nil
}

type GetCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

type GetCourseOutput struct {
	Body *Course
}

func (h *Handler) GetCourse(ctx context.Context, input *GetCourseInput) (*GetCourseOutput, error) {
	course, err := h.courseService.GetCourseByID(ctx, input.CourseID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if course == nil {
		return nil, huma.Error404NotFound("")
	}
	return &GetCourseOutput{Body: course}, nil
}

type PatchCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
	Body     UpdateCourse
}

type PatchCourseOutput struct {
	Body *Course
}

func (h *Handler) PatchCourse(ctx context.Context, input *PatchCourseInput) (*PatchCourseOutput, error) {
	course, err := h.courseService.UpdateCourseByID(ctx, input.CourseID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	return &PatchCourseOutput{Body: course}, nil
}

type DeleteCourseInput struct {
	CourseID uuid.UUID `path:"course_id"`
}

func (h *Handler) DeleteCourse(ctx context.Context, input *DeleteCourseInput) (*struct{}, error) {
	linkedBlocks, err := h.blockService.GetAllBlocksByCourseID(ctx, input.CourseID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	for _, block := range linkedBlocks {
		pos := block.Position
		updatedBlock := blocks.UpdateBlock{
			CourseID: uuid.Nil,
			Data:     block.Data,
			Position: &pos,
		}
		_, err := h.blockService.UpdateBlockByID(ctx, block.ID, &updatedBlock)
		if err != nil {
			return nil, huma.Error500InternalServerError("")
		}
	}

	if err := h.courseService.DeleteCourseByID(ctx, input.CourseID); err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	return nil, nil
}

type CreateInviteInput struct {
	Body CreateInvite
}

type CreateInviteOutput struct {
	Body *Invite
}

func (h *Handler) CreateInvite(ctx context.Context, input *CreateInviteInput) (*CreateInviteOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	invite, err := h.courseService.CreateInvite(ctx, &input.Body, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}

	return &CreateInviteOutput{Body: invite}, nil
}

type GetInviteDetailsInput struct {
	InviteID uuid.UUID `path:"invite_id"`
}

type GetInviteDetailsOutput struct {
	Body *InviteDetails
}

func (h *Handler) GetInviteDetails(ctx context.Context, input *GetInviteDetailsInput) (*GetInviteDetailsOutput, error) {
	details, err := h.courseService.GetInviteDetails(ctx, input.InviteID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
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

	courseID, err := h.courseService.JoinCourseByInvite(ctx, input.InviteID, userID)
	if err != nil {
		switch err {
		case ErrInviteNotFound:
			return nil, huma.Error404NotFound(err.Error())
		case ErrInviteRevoked, ErrInviteExpired:
			return nil, huma.NewError(http.StatusGone, "")
		case ErrAlreadyMember:
			return nil, huma.Error409Conflict("")
		default:
			return nil, huma.Error500InternalServerError("")
		}
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

	courseMember, err := h.courseService.GetCourseMember(ctx, userID, input.CourseID)
	if err != nil {
		return nil, huma.Error500InternalServerError("")
	}
	if courseMember == nil || !courseMember.IsActive {
		return nil, huma.Error404NotFound("")
	}

	return &GetUserRoleInCourseOutput{Body: &UserRoleResponse{Role: courseMember.Role}}, nil
}
