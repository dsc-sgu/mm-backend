package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/sshkeys"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/content"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
	"github.com/dsc-sgu/mm-backend/pkg/middleware"
)

func SetupRoutes(
	api huma.API,
	blockHandler *blocks.Handler,
	contentHandler *content.Handler,
	courseHandler *courses.Handler,
	disciplineHandler *disciplines.Handler,
	userHandler *users.Handler,
	sshKeyHandler *sshkeys.Handler,
	attemptHandler *attempt.Handler,
	taskHandler *tasks.Handler,
	sessionRepo session.Repo,
	cfg *config.Config,
) {
	public := huma.NewGroup(api, "")
	private := huma.NewGroup(api, "")
	if cfg.EnableAuth {
		zap.L().Info("Authorization is enabled")
		private.UseMiddleware(middleware.AuthMiddleware(sessionRepo))
	} else {
		zap.L().Info("Authorization is disabled")
		private.UseMiddleware(middleware.FakeAuthMiddleware())
	}

	setupUserRoutes(public, private, userHandler)
	setupSSHKeyRoutes(private, sshKeyHandler)
	setupAttemptRoutes(private, attemptHandler)
	setupTaskRoutes(private, taskHandler)
	setupBlockRoutes(private, blockHandler, contentHandler)
	setupCourseRoutes(private, courseHandler)
	setupDisciplineRoutes(private, disciplineHandler)
}

func setupUserRoutes(public, private huma.API, uc *users.Handler) {
	huma.Register(public, huma.Operation{
		Method: http.MethodPost, Path: "/auth/login",
		Summary: "Login user", DefaultStatus: http.StatusOK,
		Tags: []string{"Auth"},
	}, uc.Login)

	huma.Register(public, huma.Operation{
		Method: http.MethodPost, Path: "/auth/register",
		Summary: "Register new user", DefaultStatus: http.StatusCreated,
		Tags: []string{"Auth"},
	}, uc.Register)

	huma.Register(private, huma.Operation{
		Method: http.MethodPost, Path: "/auth/logout",
		Summary: "Logout user", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Auth"},
	}, uc.Logout)

	huma.Register(private, huma.Operation{
		Method: http.MethodGet, Path: "/auth/session",
		Summary: "Get session", DefaultStatus: http.StatusOK,
		Tags: []string{"Auth"},
	}, uc.GetSession)
}

func setupBlockRoutes(api huma.API, bh *blocks.Handler, ch *content.Handler) {
	const blockPath = "/courses/{course_id}/snapshots/{snapshot_id}/blocks"

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/blocks",
		Summary: "Create block in the current course draft", DefaultStatus: http.StatusCreated,
		Tags: []string{"Block"},
	}, ch.CreateBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: blockPath + "/{block_id}",
		Summary: "Get block by id", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.GetBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: blockPath + "/{block_id}",
		Summary: "Update existing block", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, ch.PatchBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: blockPath + "/{block_id}/move",
		Summary: "Move block position in snapshot", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.MoveBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: blockPath + "/{block_id}",
		Summary: "Delete block", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Block"},
	}, bh.DeleteBlock)
}

func setupCourseRoutes(api huma.API, ch *courses.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses",
		Summary: "Create new course", DefaultStatus: http.StatusCreated,
		Tags: []string{"Course"},
	}, ch.CreateCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}",
		Summary: "Get existing course by id", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.GetCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/content",
		Summary: "Get course with content", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.GetCourseContent)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/snapshots",
		Summary: "Get course snapshots timeline", DefaultStatus: http.StatusOK,
		Tags: []string{"Course editing"},
	}, ch.GetCourseSnapshots)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/snapshots/{snapshot_id}",
		Summary: "Get metadata for a single snapshot", DefaultStatus: http.StatusOK,
		Tags: []string{"Course editing"},
	}, ch.GetCourseSnapshot)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/snapshots/{snapshot_id}/blocks",
		Summary: "Get all blocks in a specific snapshot", DefaultStatus: http.StatusOK,
		Tags: []string{"Course editing"},
	}, ch.GetSnapshotBlocks)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/lock",
		Summary: "Acquire course editing lock and init draft", DefaultStatus: http.StatusOK,
		Tags: []string{"Course editing"},
	}, ch.LockCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/heartbeat",
		Summary: "Refresh course editing lock", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Course editing"},
	}, ch.Heartbeat)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/snapshots/switch",
		Summary: "Switch current draft content to target snapshot content", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Course editing"},
	}, ch.SwitchSnapshot)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/publish",
		Summary: "Publish draft snapshot to course", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Course editing"},
	}, ch.PublishDraft)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/cancel-edit",
		Summary: "Cancel course editing session", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Course editing"},
	}, ch.CancelEdit)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses",
		Summary: "Get paginated courses", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.GetPaginatedCourses)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: "/courses/{course_id}",
		Summary: "Update existing course", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.PatchCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/courses/{course_id}",
		Summary: "Delete course", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Course"},
	}, ch.DeleteCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/{course_id}/invites",
		Summary: "Create invite link for course", DefaultStatus: http.StatusCreated,
		Tags: []string{"Course Invites"},
	}, ch.CreateInvite)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/invites",
		Summary: "List invite links for course", DefaultStatus: http.StatusOK,
		Tags: []string{"Course Invites"},
	}, ch.GetCourseInvites)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/invites/{invite_id}",
		Summary: "Get invite link details", DefaultStatus: http.StatusOK,
		Tags: []string{"Course Invites"},
	}, ch.GetInviteDetails)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/invites/{invite_id}/join",
		Summary: "Join course by invite link", DefaultStatus: http.StatusOK,
		Tags: []string{"Course Invites"},
	}, ch.JoinCourseByInvite)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/{course_id}/role",
		Summary: "Get role of current user in course", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.GetUserRoleInCourse)
}

func setupDisciplineRoutes(api huma.API, dh *disciplines.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/disciplines",
		Summary: "Create new discipline", DefaultStatus: http.StatusCreated,
		Tags: []string{"Discipline"},
	}, dh.CreateDiscipline)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/disciplines/{discipline_id}",
		Summary: "Get discipline by id", DefaultStatus: http.StatusOK,
		Tags: []string{"Discipline"},
	}, dh.GetDiscipline)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: "/disciplines/{discipline_id}",
		Summary: "Update existing discipline", DefaultStatus: http.StatusOK,
		Tags: []string{"Discipline"},
	}, dh.PatchDiscipline)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/disciplines/{discipline_id}",
		Summary: "Delete discipline", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Discipline"},
	}, dh.DeleteDiscipline)
}

func setupSSHKeyRoutes(api huma.API, handler *sshkeys.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/auth/ssh-keys",
		Summary: "Add SSH key", DefaultStatus: http.StatusCreated,
		Tags: []string{"Auth"},
	}, handler.Add)
	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/auth/ssh-keys/{fingerprint}",
		Summary: "Delete SSH key", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Auth"},
	}, handler.Delete)
}

func setupAttemptRoutes(api huma.API, handler *attempt.Handler) {
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/attempts/diff", Summary: "Get attempt diff", Tags: []string{"Attempt"}}, handler.GetDiff)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/attempts/{task_id}/{participant_id}", Summary: "Get attempts", Tags: []string{"Attempt"}}, handler.GetAttempts)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/attempts", Summary: "Push attempt", DefaultStatus: http.StatusCreated, Tags: []string{"Attempt"}}, handler.PushAttempt)
}

func setupTaskRoutes(api huma.API, handler *tasks.Handler) {
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tasks/{group_id}", Summary: "Get task group", Tags: []string{"Task"}}, handler.GetTaskGroup)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks", Summary: "Create task group", DefaultStatus: http.StatusCreated, Tags: []string{"Task"}}, handler.CreateTaskGroup)
	huma.Register(api, huma.Operation{Method: http.MethodPatch, Path: "/tasks/{group_id}", Summary: "Update task group", Tags: []string{"Task"}}, handler.PatchTaskGroup)
	huma.Register(api, huma.Operation{Method: http.MethodDelete, Path: "/tasks/{group_id}", Summary: "Delete task group", DefaultStatus: http.StatusNoContent, Tags: []string{"Task"}}, handler.DeleteTaskGroup)
	huma.Register(api, huma.Operation{Method: http.MethodPost, Path: "/tasks/{group_id}/template", Summary: "Upload task template", DefaultStatus: http.StatusNoContent, Tags: []string{"Task"}}, handler.UploadTemplate)
	huma.Register(api, huma.Operation{Method: http.MethodGet, Path: "/tasks/{group_id}/tasks", Summary: "Get tasks", Tags: []string{"Task"}}, handler.GetTasks)
}
