package tasks

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type Handler struct {
	taskSvc *Service
}

func NewHandler(taskSvc *Service) *Handler {
	return &Handler{taskSvc: taskSvc}
}

type GetTaskGroupInput struct {
	GroupID string `path:"group_id"`
}

type GetTaskGroupOutput struct {
	Body *TaskGroupWithTasks
}

func (h *Handler) GetTaskGroup(ctx context.Context, input *GetTaskGroupInput) (*GetTaskGroupOutput, error) {
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	tg, err := h.taskSvc.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if tg == nil {
		return nil, huma.Error404NotFound("task group not found")
	}

	tasksList, err := h.taskSvc.GetTasks(ctx, groupID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	result := TaskGroupWithTasks{
		TaskGroup: *tg,
		Tasks:     tasksList,
	}

	return &GetTaskGroupOutput{Body: &result}, nil
}

type CreateTaskGroupInput struct {
	Body CreateTaskGroup
}

type CreateTaskGroupOutput struct {
	Body *CreateTaskGroupResponse
}

func (h *Handler) CreateTaskGroup(ctx context.Context, input *CreateTaskGroupInput) (*CreateTaskGroupOutput, error) {
	tg, err := h.taskSvc.CreateTaskGroup(ctx, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
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
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	tg, err := h.taskSvc.UpdateTaskGroup(ctx, groupID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &PatchTaskGroupOutput{Body: tg}, nil
}

type DeleteTaskGroupInput struct {
	GroupID string `path:"group_id"`
}

func (h *Handler) DeleteTaskGroup(ctx context.Context, input *DeleteTaskGroupInput) (*struct{}, error) {
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	if err := h.taskSvc.DeleteTaskGroup(ctx, groupID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

type UploadTemplateInput struct {
	GroupID string `path:"group_id"`
	RawBody []byte
}

func (h *Handler) UploadTemplate(ctx context.Context, input *UploadTemplateInput) (*struct{}, error) {
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	if len(input.RawBody) == 0 {
		return nil, huma.Error400BadRequest("empty body")
	}

	if err := h.taskSvc.UploadTemplate(ctx, groupID, input.RawBody); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
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
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	taskList, err := h.taskSvc.GetTasks(ctx, groupID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetTasksOutput{Body: taskList}, nil
}

type PatchTaskInput struct {
	GroupID string `path:"group_id"`
	TaskID  string `path:"task_id"`
	Body    UpdateTask
}

type PatchTaskOutput struct {
	Body *Task
}

func (h *Handler) PatchTask(ctx context.Context, input *PatchTaskInput) (*PatchTaskOutput, error) {
	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing task_id: " + err.Error())
	}

	task, err := h.taskSvc.UpdateTask(ctx, taskID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &PatchTaskOutput{Body: task}, nil
}
