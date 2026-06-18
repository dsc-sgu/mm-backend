package tasks

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

type Handler struct {
	taskSvc  *Service
	blockSvc *blocks.Service
}

func NewHandler(taskSvc *Service, blockSvc *blocks.Service) *Handler {
	return &Handler{taskSvc: taskSvc, blockSvc: blockSvc}
}

type GetTaskGroupInput struct {
	BlockID string `path:"block_id"`
}

type GetTaskGroupOutput struct {
	Body *TaskGroupWithTasks
}

func (h *Handler) GetTaskGroup(ctx context.Context, input *GetTaskGroupInput) (*GetTaskGroupOutput, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	tg, err := h.taskSvc.GetTaskGroupByBlockID(ctx, blockID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if tg == nil {
		return nil, huma.Error404NotFound("task group not found")
	}

	tasksList, err := h.taskSvc.GetTasks(ctx, blockID)
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
	createBlock := blocks.CreateBlock{
		CourseID:  input.Body.CourseID,
		BlockType: "task_group",
		Data:      input.Body.Data,
	}
	block, err := h.blockSvc.CreateBlock(ctx, &createBlock)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	tg, err := h.taskSvc.CreateTaskGroup(ctx, block.ID, &CreateTaskGroup{
		CourseID: block.CourseID,
		Name:     input.Body.Name,
		Data:     input.Body.Data,
	})
	if err != nil {
		_ = h.blockSvc.DeleteBlockByID(ctx, block.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &CreateTaskGroupOutput{
		Body: &CreateTaskGroupResponse{BlockID: tg.BlockID},
	}, nil
}

type PatchTaskGroupInput struct {
	BlockID string `path:"block_id"`
	Body    UpdateTaskGroup
}

type PatchTaskGroupOutput struct {
	Body *TaskGroup
}

func (h *Handler) PatchTaskGroup(ctx context.Context, input *PatchTaskGroupInput) (*PatchTaskGroupOutput, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	tg, err := h.taskSvc.UpdateTaskGroup(ctx, blockID, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &PatchTaskGroupOutput{Body: tg}, nil
}

type DeleteTaskGroupInput struct {
	BlockID string `path:"block_id"`
}

func (h *Handler) DeleteTaskGroup(ctx context.Context, input *DeleteTaskGroupInput) (*struct{}, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	if err := h.taskSvc.DeleteTaskGroup(ctx, blockID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if err := h.blockSvc.DeleteBlockByID(ctx, blockID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

type UploadTemplateInput struct {
	BlockID string `path:"block_id"`
	RawBody []byte
}

func (h *Handler) UploadTemplate(ctx context.Context, input *UploadTemplateInput) (*struct{}, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	if len(input.RawBody) == 0 {
		return nil, huma.Error400BadRequest("empty body")
	}

	if err := h.taskSvc.UploadTemplate(ctx, blockID, input.RawBody); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

type GetTasksInput struct {
	BlockID string `path:"block_id"`
}

type GetTasksOutput struct {
	Body []*Task
}

func (h *Handler) GetTasks(ctx context.Context, input *GetTasksInput) (*GetTasksOutput, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	taskList, err := h.taskSvc.GetTasks(ctx, blockID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetTasksOutput{Body: taskList}, nil
}

type CreateTaskInput struct {
	BlockID string `path:"block_id"`
	Body    CreateTask
}

type CreateTaskOutput struct {
	Body *CreateTaskResponse
}

func (h *Handler) CreateTask(ctx context.Context, input *CreateTaskInput) (*CreateTaskOutput, error) {
	blockID, err := uuid.Parse(input.BlockID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing block_id: " + err.Error())
	}

	input.Body.TaskGroupID = blockID

	task, err := h.taskSvc.CreateTask(ctx, &input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &CreateTaskOutput{
		Body: &CreateTaskResponse{ID: task.ID},
	}, nil
}

type PatchTaskInput struct {
	BlockID string `path:"block_id"`
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

type DeleteTaskInput struct {
	BlockID string `path:"block_id"`
	TaskID  string `path:"task_id"`
}

func (h *Handler) DeleteTask(ctx context.Context, input *DeleteTaskInput) (*struct{}, error) {
	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing task_id: " + err.Error())
	}

	if err := h.taskSvc.DeleteTask(ctx, taskID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}
