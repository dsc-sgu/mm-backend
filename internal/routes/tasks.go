package routes

import (
	"fmt"
	"io"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

type TaskGroupController struct {
	taskSvc  *tasks.Service
	blockSvc *blocks.Service
}

func NewTaskGroupController(taskSvc *tasks.Service, blockSvc *blocks.Service) *TaskGroupController {
	return &TaskGroupController{
		taskSvc:  taskSvc,
		blockSvc: blockSvc,
	}
}

func (c *TaskGroupController) CreateTaskGroup(
	ctx fuego.ContextWithBody[tasks.CreateTaskGroup],
) (*tasks.CreateTaskGroupResponse, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	// Create block first
	createBlock := blocks.CreateBlock{
		CourseID:  body.CourseID,
		BlockType: "task_group",
		Data:      body.Data,
	}
	block, err := c.blockSvc.CreateBlock(ctx.Context(), &createBlock)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	// Create task_group referencing the block
	tg, err := c.taskSvc.CreateTaskGroup(ctx.Context(), block.ID, &tasks.CreateTaskGroup{
		CourseID: block.CourseID,
		Name:     body.Name,
		Data:     body.Data,
	})
	if err != nil {
		// Cleanup: delete the block if task_group creation fails
		_ = c.blockSvc.DeleteBlockByID(ctx.Context(), block.ID)
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := tasks.CreateTaskGroupResponse{
		BlockID: tg.BlockID,
	}

	return &response, nil
}

func (c *TaskGroupController) GetTaskGroup(
	ctx fuego.ContextNoBody,
) (*tasks.TaskGroupWithTasks, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	tg, err := c.taskSvc.GetTaskGroupByBlockID(ctx.Context(), blockID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}
	if tg == nil {
		return nil, fuego.NotFoundError{Detail: "task group not found"}
	}

	tasksList, err := c.taskSvc.GetTasks(ctx.Context(), blockID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	result := tasks.TaskGroupWithTasks{
		TaskGroup: *tg,
		Tasks:     tasksList,
	}

	return &result, nil
}

func (c *TaskGroupController) PatchTaskGroup(
	ctx fuego.ContextWithBody[tasks.UpdateTaskGroup],
) (*tasks.TaskGroup, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	tg, err := c.taskSvc.UpdateTaskGroup(ctx.Context(), blockID, &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return tg, nil
}

func (c *TaskGroupController) DeleteTaskGroup(ctx fuego.ContextNoBody) (any, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	// Delete task_group (CASCADE will delete tasks)
	if err := c.taskSvc.DeleteTaskGroup(ctx.Context(), blockID); err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	// Delete the block
	if err := c.blockSvc.DeleteBlockByID(ctx.Context(), blockID); err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}

func (c *TaskGroupController) UploadTemplate(ctx fuego.ContextNoBody) (any, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	// Check task group exists
	tg, err := c.taskSvc.GetTaskGroupByBlockID(ctx.Context(), blockID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}
	if tg == nil {
		return nil, fuego.NotFoundError{Detail: "task group not found"}
	}

	// Read zip data from body
	zipData, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("reading body: %w", err).Error(),
		}
	}

	if len(zipData) == 0 {
		return nil, fuego.BadRequestError{Title: "EMPTY_BODY"}
	}

	// TODO: handle template zip saving
	// For now, just acknowledge
	return nil, nil
}

// Tasks within a group

func (c *TaskGroupController) CreateTask(
	ctx fuego.ContextWithBody[tasks.CreateTask],
) (*tasks.CreateTaskResponse, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	body.TaskGroupID = blockID

	task, err := c.taskSvc.CreateTask(ctx.Context(), &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	response := tasks.CreateTaskResponse{
		ID: task.ID,
	}

	return &response, nil
}

func (c *TaskGroupController) PatchTask(
	ctx fuego.ContextWithBody[tasks.UpdateTask],
) (*tasks.Task, error) {
	taskID, err := uuid.Parse(ctx.PathParam("task_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	task, err := c.taskSvc.UpdateTask(ctx.Context(), taskID, &body)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return task, nil
}

func (c *TaskGroupController) DeleteTask(ctx fuego.ContextNoBody) (any, error) {
	taskID, err := uuid.Parse(ctx.PathParam("task_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	if err := c.taskSvc.DeleteTask(ctx.Context(), taskID); err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return nil, nil
}

func (c *TaskGroupController) GetTasks(ctx fuego.ContextNoBody) ([]*tasks.Task, error) {
	blockID, err := uuid.Parse(ctx.PathParam("block_id"))
	if err != nil {
		return nil, fuego.BadRequestError{
			Detail: fmt.Errorf("parsing UUID: %w", err).Error(),
		}
	}

	taskList, err := c.taskSvc.GetTasks(ctx.Context(), blockID)
	if err != nil {
		return nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return taskList, nil
}
