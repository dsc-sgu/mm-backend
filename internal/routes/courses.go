package routes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/lock"
)

const (
	lockTTL = 60 * time.Second
)

type CourseController struct {
	courseService *courses.Service
	blockService  *blocks.Service
	lockManager   lock.Manager
}

func NewCourseController(
	courseService *courses.Service,
	blockService *blocks.Service,
	lockManager lock.Manager,
) *CourseController {
	return &CourseController{
		courseService: courseService,
		blockService:  blockService,
		lockManager:   lockManager,
	}
}

func (c *CourseController) CreateCourse(
	ctx fuego.ContextWithBody[courses.CreateCourse],
) (*courses.CourseIDResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	course, err := c.courseService.CreateCourse(ctx.Context(), &body, userID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := courses.CourseIDResponse{
		ID: course.ID,
	}

	return &response, nil
}

// Other endpoints rewrote using fuego
func (c *CourseController) GetPaginatedCourses(
	ctx fuego.ContextNoBody,
) ([]courses.Course, error) {
	pathLimit := ctx.QueryParam("limit")
	pathID := ctx.QueryParam("last_id")

	limit, err := strconv.Atoi(pathLimit)
	if err != nil {
		return nil, fuego.BadRequestError{Detail: err.Error()}
	}

	id, err := uuid.Parse(pathID)
	if err != nil {
		if pathID == "" {
			id = uuid.Nil
		} else {
			return nil, fuego.BadRequestError{
				Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
			}
		}
	}

	course, err := c.courseService.GetPaginatedCourses(ctx.Context(), limit, id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return course, nil
}

func (c *CourseController) GetCourse(
	ctx fuego.ContextNoBody,
) (*courses.Course, error) {
	pathID := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	course, err := c.courseService.GetCourseByID(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return course, nil
}

func (c *CourseController) UpdateCourse(
	ctx fuego.ContextWithBody[courses.UpdateCourseRequest],
) (*courses.Course, error) {
	pathID := ctx.PathParam("course_id")

	courseID, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{}
	}

	// Check lock
	lockerID, err := c.lockManager.GetLocker(ctx.Context(), courseID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}
	if lockerID != userID {
		return nil, fuego.HTTPError{
			Status: http.StatusLocked,
			Detail: "course is locked by another user",
		}
	}

	course, err := c.courseService.UpdateCourse(ctx.Context(), courseID, &body)
	if err != nil {
		if errors.Is(err, courses.ErrVersionMismatch) {
			return nil, fuego.ConflictError{Detail: err.Error()}
		}
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	// Release lock after successful update
	if err := c.lockManager.Release(ctx.Context(), courseID, userID); err != nil {
		// Log the error but don't fail the request, as the primary operation succeeded.
		zap.L().Error("failed to release lock after update", zap.Error(err))
	}

	return course, nil
}

func (c *CourseController) LockCourse(ctx fuego.ContextNoBody) (any, error) {
	pathID := ctx.PathParam("course_id")
	courseID, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{Detail: "invalid course id"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{}
	}

	// TODO: check if user is a teacher of the course

	lockerID, err := c.lockManager.GetLocker(ctx.Context(), courseID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	if lockerID != uuid.Nil && lockerID != userID {
		return nil, fuego.HTTPError{
			Status: http.StatusLocked,
			Detail: "course is locked by another user",
		}
	}

	ok, err := c.lockManager.Acquire(ctx.Context(), courseID, userID, lockTTL)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	if !ok {
		return nil, fuego.HTTPError{
			Status: http.StatusLocked,
			Detail: "failed to acquire lock",
		}
	}

	return nil, nil
}

func (c *CourseController) Heartbeat(ctx fuego.ContextNoBody) (any, error) {
	pathID := ctx.PathParam("course_id")
	courseID, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{Detail: "invalid course id"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{}
	}

	err = c.lockManager.Heartbeat(ctx.Context(), courseID, userID, lockTTL)
	if err != nil {
		if errors.Is(err, lock.ErrLockNotAcquired) {
			return nil, fuego.ConflictError{
				Detail: "lock is not acquired or expired",
			}
		}
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}

func (c *CourseController) UnlockCourse(ctx fuego.ContextNoBody) (any, error) {
	pathID := ctx.PathParam("course_id")
	courseID, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{Detail: "invalid course id"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{}
	}

	err = c.lockManager.Release(ctx.Context(), courseID, userID)
	if err != nil {
		if errors.Is(err, lock.ErrLockNotAcquired) {
			// It's fine if the lock was already released or expired
			return nil, nil
		}
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}

func (c *CourseController) DeleteCourse(
	ctx fuego.ContextNoBody,
) (any, error) {
	pathID := ctx.PathParam("course_id")

	id, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	blockID, err := uuid.Parse(pathID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	err = c.courseService.DeleteCourseByID(ctx.Context(), id)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}
	linkedBlocks, err := c.blockService.GetAllBlocksByCourseID(
		ctx.Context(),
		blockID,
	)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	for _, block := range linkedBlocks {
		updatedBlock := blocks.UpdateBlock{
			CourseID: uuid.Nil,
			Data:     block.Data,
			Position: block.Position,
		}
		_, err := c.blockService.UpdateBlockByID(
			ctx.Context(),
			block.ID,
			&updatedBlock,
		)
		if err != nil {
			return nil, fuego.InternalServerError{Detail: err.Error()}
		}
	}

	return nil, nil
}

func (c *CourseController) CreateInvite(
	ctx fuego.ContextWithBody[courses.CreateInvite],
) (*courses.Invite, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	invite, err := c.courseService.CreateInvite(
		ctx.Context(),
		&body,
		userID,
	)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return invite, nil
}

func (c *CourseController) GetInviteDetails(
	ctx fuego.ContextNoBody,
) (*courses.InviteDetails, error) {
	pathInviteID := ctx.PathParam("invite_id")

	inviteID, err := uuid.Parse(pathInviteID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	details, err := c.courseService.GetInviteDetails(ctx.Context(), inviteID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return details, nil
}

func (c *CourseController) JoinCourseByInvite(
	ctx fuego.ContextNoBody,
) (*courses.CourseIDResponse, error) {
	pathInviteID := ctx.PathParam("invite_id")

	inviteID, err := uuid.Parse(pathInviteID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	err = c.courseService.JoinCourseByInvite(ctx.Context(), inviteID, userID)
	if err != nil {
		switch err {
		case courses.ErrInviteNotFound:
			return nil, fuego.NotFoundError{Detail: err.Error()}
		case courses.ErrInviteRevoked, courses.ErrInviteExpired:
			return nil, fuego.HTTPError{
				Detail: err.Error(),
				Status: http.StatusGone,
			}
		case courses.ErrAlreadyMember:
			return nil, fuego.ConflictError{Detail: err.Error()}
		default:
			return nil, fuego.InternalServerError{Detail: err.Error()}
		}
	}

	response := courses.CourseIDResponse{
		ID: userID,
	}

	return &response, nil
}

func (c *CourseController) GetUserRoleInCourse(
	ctx fuego.ContextNoBody,
) (*courses.UserRoleResponse, error) {
	pathCourseID := ctx.PathParam("course_id")

	courseID, err := uuid.Parse(pathCourseID)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	userID := session.UserIDFromContext(ctx.Context())
	if userID == uuid.Nil {
		return nil, fuego.UnauthorizedError{Title: "WRONG_CREDENTIALS"}
	}

	courseMember, err := c.courseService.GetCourseMember(
		ctx.Context(),
		userID,
		courseID,
	)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}
	if !courseMember.IsActive {
		return nil, fuego.NotFoundError{Detail: "course member not found"}
	}

	response := courses.UserRoleResponse{
		Role: courseMember.Role,
	}

	return &response, nil
}
