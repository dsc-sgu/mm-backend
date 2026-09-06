package tasks

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

type Handler struct {
	taskSvc *Service
}

func NewHandler(taskSvc *Service) *Handler {
	return &Handler{taskSvc: taskSvc}
}

func handleServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrTaskGroupNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, membership.ErrNotFound),
		errors.Is(err, membership.ErrPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type GetTaskGroupInput struct {
	GroupID string `path:"group_id"`
}

type GetTaskGroupOutput struct {
	Body *TaskGroupWithTasks
}

func (h *Handler) GetTaskGroup(ctx context.Context, input *GetTaskGroupInput) (*GetTaskGroupOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	result, err := h.taskSvc.GetTaskGroup(ctx, userID, sessionID, groupID)
	if err != nil {
		return nil, handleServiceError(err)
	}
	if result == nil {
		return nil, huma.Error404NotFound("task group not found")
	}

	return &GetTaskGroupOutput{Body: result}, nil
}

type CreateTaskGroupInput struct {
	Body CreateTaskGroup
}

type CreateTaskGroupOutput struct {
	Body *CreateTaskGroupResponse
}

func (h *Handler) CreateTaskGroup(ctx context.Context, input *CreateTaskGroupInput) (*CreateTaskGroupOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	tg, err := h.taskSvc.CreateTaskGroup(ctx, userID, &input.Body)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &CreateTaskGroupOutput{
		Body: &CreateTaskGroupResponse{ID: tg.ID},
	}, nil
}

type PatchTaskGroupInput struct {
	GroupID string `path:"group_id"`
	Body    UpdateTaskGroup
}

type PatchTaskGroupOutput struct {
	Body *TaskGroup
}

func (h *Handler) PatchTaskGroup(ctx context.Context, input *PatchTaskGroupInput) (*PatchTaskGroupOutput, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	tg, err := h.taskSvc.UpdateTaskGroup(ctx, userID, groupID, &input.Body)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &PatchTaskGroupOutput{Body: tg}, nil
}

type DeleteTaskGroupInput struct {
	GroupID string `path:"group_id"`
}

func (h *Handler) DeleteTaskGroup(ctx context.Context, input *DeleteTaskGroupInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	if err := h.taskSvc.DeleteTaskGroup(ctx, userID, groupID); err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type UploadTemplateInput struct {
	GroupID string `path:"group_id"`
	RawBody []byte
}

func (h *Handler) UploadTemplate(ctx context.Context, input *UploadTemplateInput) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	if len(input.RawBody) == 0 {
		return nil, huma.Error400BadRequest("empty body")
	}

	if err := h.taskSvc.UploadTemplate(ctx, userID, groupID, input.RawBody); err != nil {
		return nil, handleServiceError(err)
	}

	return nil, nil
}

type GetTasksInput struct {
	GroupID string `path:"group_id"`
}

type GetTasksOutput struct {
	Body []*Task
}

func (h *Handler) GetTasks(ctx context.Context, input *GetTasksInput) (*GetTasksOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	taskList, err := h.taskSvc.GetTasks(ctx, userID, sessionID, groupID)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &GetTasksOutput{Body: taskList}, nil
}
