package attempt

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

type Handler struct {
	svc     *Service
	taskSvc *tasks.Service
}

func NewHandler(svc *Service, taskSvc *tasks.Service) *Handler {
	return &Handler{svc, taskSvc}
}

type GetDiffInput struct {
	ID1 string `query:"id1"`
	ID2 string `query:"id2"`
}

type GetDiffOutput struct {
	Body []string
}

func (h *Handler) GetDiff(ctx context.Context, input *GetDiffInput) (*GetDiffOutput, error) {
	id1, err := uuid.Parse(input.ID1)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing id1: " + err.Error())
	}

	id2, err := uuid.Parse(input.ID2)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing id2: " + err.Error())
	}

	diff, err := h.svc.GetDiff(id1, id2)
	if err != nil {
		return nil, huma.Error400BadRequest("cannot make diff: " + err.Error())
	}

	return &GetDiffOutput{Body: diff}, nil
}

type PushAttemptInput struct {
	CourseID    string `query:"courseID"`
	TaskGroupID string `query:"taskGroupID"`
	TaskID      string `query:"taskID"`
	RawBody     []byte
}

type PushAttemptOutput struct {
	Body struct {
		CommitHash string `json:"commitHash"`
	}
}

func (h *Handler) PushAttempt(ctx context.Context, input *PushAttemptInput) (*PushAttemptOutput, error) {
	courseID, err := uuid.Parse(input.CourseID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing courseID: " + err.Error())
	}

	taskGroupID, err := uuid.Parse(input.TaskGroupID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing taskGroupID: " + err.Error())
	}

	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing taskID: " + err.Error())
	}

	participantID := session.UserIDFromContext(ctx)
	if participantID == uuid.Nil {
		return nil, huma.Error401Unauthorized("")
	}

	task, err := h.taskSvc.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, huma.Error400BadRequest("task not found: " + err.Error())
	}
	if task.TaskGroupID != taskGroupID {
		return nil, huma.Error400BadRequest("task does not belong to the given task group")
	}

	commitHash, err := h.svc.PushAttempt(courseID, taskGroupID, task.ID, participantID, input.RawBody)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	out := &PushAttemptOutput{}
	out.Body.CommitHash = commitHash
	return out, nil
}

type GetAttemptsInput struct {
	TaskID        string `path:"task_id"`
	ParticipantID string `path:"participant_id"`
}

type GetAttemptsOutput struct {
	Body []Attempt
}

func (h *Handler) GetAttempts(ctx context.Context, input *GetAttemptsInput) (*GetAttemptsOutput, error) {
	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing task_id: " + err.Error())
	}

	participantID, err := uuid.Parse(input.ParticipantID)
	if err != nil {
		return nil, huma.Error400BadRequest("parsing participant_id: " + err.Error())
	}

	attempts, err := h.svc.GetAttempts(taskID, participantID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetAttemptsOutput{Body: attempts}, nil
}
