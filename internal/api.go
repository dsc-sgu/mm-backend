package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/dsc-sgu/mm-backend/pkg/middleware"
)

func SetupRoutes(
	api huma.API,
	blockHandler *blocks.Handler,
	courseHandler *courses.Handler,
	disciplineHandler *disciplines.Handler,
	userHandler *users.Handler,
	gitHandler *git.Handler,
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
	setupBlockRoutes(private, blockHandler)
	setupCourseRoutes(private, courseHandler)
	setupDisciplineRoutes(private, disciplineHandler)
	setupGitRoutes(private, gitHandler)
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

func setupBlockRoutes(api huma.API, bh *blocks.Handler) {
	const blockPath = "/courses/{course_id}/snapshots/{snapshot_id}/blocks"

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: blockPath + "/{block_id}",
		Summary: "Get block by id", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.GetBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: blockPath,
		Summary: "Create new block on snapshot", DefaultStatus: http.StatusCreated,
		Tags: []string{"Block"},
	}, bh.CreateBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: blockPath + "/{block_id}",
		Summary: "Update existing block", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.PatchBlock)

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

func setupGitRoutes(api huma.API, gh *git.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/git/add_key",
		Summary: "Add new SSH key", DefaultStatus: http.StatusAccepted,
		Tags: []string{"Git"},
	}, gh.AddSshKey)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/git/delete_key",
		Summary: "Delete SSH key", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Git"},
	}, gh.DeleteSSHKey)
}

func setupAttemptRoutes(api huma.API, ah *attempt.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/attempts/diff",
		Summary: "Get attempt diff by two ids", DefaultStatus: http.StatusOK,
		Tags: []string{"Attempt"},
	}, ah.GetDiff)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/attempts/{task_id}/{participant_id}",
		Summary: "Get all user attempts on task", DefaultStatus: http.StatusOK,
		Tags: []string{"Attempt"},
	}, ah.GetAttempts)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/attempts",
		Summary:       "Push attempt via zip upload",
		DefaultStatus: http.StatusCreated,
		Tags:          []string{"Attempt"},
	}, ah.PushAttempt)
}

func setupTaskRoutes(api huma.API, th *tasks.Handler) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/tasks/{block_id}",
		Summary: "Get task group with its tasks", DefaultStatus: http.StatusOK,
		Tags: []string{"Task"},
	}, th.GetTaskGroup)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/tasks",
		Summary: "Create new task group", DefaultStatus: http.StatusCreated,
		Tags: []string{"Task"},
	}, th.CreateTaskGroup)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: "/tasks/{block_id}",
		Summary: "Update existing task group", DefaultStatus: http.StatusOK,
		Tags: []string{"Task"},
	}, th.PatchTaskGroup)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/tasks/{block_id}",
		Summary: "Delete task group", DefaultStatus: http.StatusNoContent,
		Tags: []string{"Task"},
	}, th.DeleteTaskGroup)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/tasks/{block_id}/template",
		Summary:       "Upload template zip",
		DefaultStatus: http.StatusAccepted,
		Tags:          []string{"Task"},
	}, th.UploadTemplate)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/tasks/{block_id}/tasks",
		Summary: "Get all tasks in a group", DefaultStatus: http.StatusOK,
		Tags: []string{"Task"},
	}, th.GetTasks)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/tasks/{block_id}/tasks",
		Summary:       "Create a new task within a group",
		DefaultStatus: http.StatusCreated,
		Tags:          []string{"Task"},
	}, th.CreateTask)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: "/tasks/{block_id}/tasks/{task_id}",
		Summary: "Update a task within a group", DefaultStatus: http.StatusOK,
		Tags: []string{"Task"},
	}, th.PatchTask)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/tasks/{block_id}/tasks/{task_id}",
		Summary:       "Delete a task from a group",
		DefaultStatus: http.StatusNoContent,
		Tags:          []string{"Task"},
	}, th.DeleteTask)
}
