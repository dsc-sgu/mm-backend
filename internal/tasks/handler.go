package tasks

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
)

type Handler struct {
	taskSvc     *Service
	blockSvc    *blocks.Service
	snapshotSvc *snapshots.Service
}

func NewHandler(
	taskSvc *Service,
	blockSvc *blocks.Service,
	snapshotSvc *snapshots.Service,
) *Handler {
	return &Handler{
		taskSvc:     taskSvc,
		blockSvc:    blockSvc,
		snapshotSvc: snapshotSvc,
	}
}

func handleBlockServiceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, blocks.ErrSnapshotNotFound),
		errors.Is(err, blocks.ErrBlockNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, blocks.ErrPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, blocks.ErrSnapshotNotDraft),
		errors.Is(err, blocks.ErrAfterBlockNotFound),
		errors.Is(err, blocks.ErrInvalidBlockForMoveAfter):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, locks.ErrLockHeldByAnother),
		errors.Is(err, locks.ErrLockNotFound),
		errors.Is(err, locks.ErrLockExpired):
		return huma.Error423Locked(err.Error())
	}
	return huma.Error500InternalServerError(err.Error())
}

type GetTaskGroupInput struct {
	GroupID string `path:"group_id"`
}

type GetTaskGroupOutput struct {
	Body *TaskGroupWithTasks
}

func (h *Handler) GetTaskGroup(
	ctx context.Context,
	input *GetTaskGroupInput,
) (*GetTaskGroupOutput, error) {
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

func (h *Handler) CreateTaskGroup(
	ctx context.Context,
	input *CreateTaskGroupInput,
) (*CreateTaskGroupOutput, error) {
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

func (h *Handler) PatchTaskGroup(
	ctx context.Context,
	input *PatchTaskGroupInput,
) (*PatchTaskGroupOutput, error) {
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

func (h *Handler) DeleteTaskGroup(
	ctx context.Context,
	input *DeleteTaskGroupInput,
) (*struct{}, error) {
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

func (h *Handler) UploadTemplate(
	ctx context.Context,
	input *UploadTemplateInput,
) (*struct{}, error) {
	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	if len(input.RawBody) == 0 {
		return nil, huma.Error400BadRequest("empty body")
	}

	if err := h.taskSvc.UploadTemplate(
		ctx,
		groupID,
		input.RawBody,
	); err != nil {
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

func (h *Handler) GetTasks(
	ctx context.Context,
	input *GetTasksInput,
) (*GetTasksOutput, error) {
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

type CreateTaskInput struct {
	GroupID string `path:"group_id"`
	Body    CreateTask
}

type CreateTaskOutput struct {
	Body *CreateTaskResponse
}

func (h *Handler) CreateTask(
	ctx context.Context,
	input *CreateTaskInput,
) (*CreateTaskOutput, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}

	// A task is a subtype of block, so first create the backing block (the unit
	// of course display) and then the task row that references it.
	tg, err := h.taskSvc.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if tg == nil {
		return nil, huma.Error404NotFound("task group not found")
	}

	// New content can only be added to the course's current draft, which
	// requires the caller to already hold the course's edit lock (see
	// POST /courses/{course_id}/lock).
	draft, err := h.snapshotSvc.FindUserDraft(ctx, tg.CourseID, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if draft == nil {
		return nil, huma.Error423Locked(
			"course is not locked for editing by this user; call POST /courses/{course_id}/lock first",
		)
	}

	block, err := h.blockSvc.CreateBlock(ctx, &blocks.CreateBlock{
		CourseID:   tg.CourseID,
		SnapshotID: draft.ID,
		BlockType:  "task",
		Data:       input.Body.Data,
	}, userID, sessionID)
	if err != nil {
		return nil, handleBlockServiceError(err)
	}

	// tasks.block_id tracks the block's stable origin_id (not its per-snapshot
	// row id), so the task keeps a single identity across future drafts.
	input.Body.BlockID = block.OriginID
	input.Body.TaskGroupID = groupID

	task, err := h.taskSvc.CreateTask(ctx, &input.Body)
	if err != nil {
		if derr := h.blockSvc.DeleteBlockByID(ctx, blocks.BlockRef{
			BlockID:    block.ID,
			CourseID:   tg.CourseID,
			SnapshotID: draft.ID,
			UserID:     userID,
			SessionID:  sessionID,
		}); derr != nil {
			return nil, huma.Error500InternalServerError(
				err.Error() + "; rollback: " + derr.Error(),
			)
		}
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &CreateTaskOutput{
		Body: &CreateTaskResponse{ID: task.ID},
	}, nil
}

type PatchTaskInput struct {
	GroupID string `path:"group_id"`
	TaskID  string `path:"task_id"`
	Body    UpdateTask
}

type PatchTaskOutput struct {
	Body *Task
}

func (h *Handler) PatchTask(
	ctx context.Context,
	input *PatchTaskInput,
) (*PatchTaskOutput, error) {
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
	GroupID string `path:"group_id"`
	TaskID  string `path:"task_id"`
}

func (h *Handler) DeleteTask(
	ctx context.Context,
	input *DeleteTaskInput,
) (*struct{}, error) {
	userID := session.UserIDFromContext(ctx)
	sessionID := session.SessionIDFromContext(ctx)
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	groupID, err := uuid.Parse(input.GroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing group_id: " + err.Error())
	}
	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing task_id: " + err.Error())
	}

	tg, err := h.taskSvc.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if tg == nil {
		return nil, huma.Error404NotFound("task group not found")
	}

	draft, err := h.snapshotSvc.FindUserDraft(ctx, tg.CourseID, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if draft == nil {
		return nil, huma.Error423Locked(
			"course is not locked for editing by this user; call POST /courses/{course_id}/lock first",
		)
	}

	// taskID is the task's stable origin_id; resolve it to the concrete
	// block row belonging to the current draft before deleting it.
	block, err := h.blockSvc.GetBlockByOriginID(ctx, taskID, draft.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if block == nil {
		return nil, huma.Error404NotFound(
			"task block not found in current draft",
		)
	}

	if err := h.taskSvc.DeleteTask(ctx, taskID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if err := h.blockSvc.DeleteBlockByID(ctx, blocks.BlockRef{
		BlockID:    block.ID,
		CourseID:   tg.CourseID,
		SnapshotID: draft.ID,
		UserID:     userID,
		SessionID:  sessionID,
	}); err != nil {
		return nil, handleBlockServiceError(err)
	}

	return nil, nil
}
