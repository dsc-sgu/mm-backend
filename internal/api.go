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
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/blocks/{block_id}",
		Summary: "Get block by id", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.GetBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/blocks/{course_id}/blocks",
		Summary: "Create new block on course", DefaultStatus: http.StatusCreated,
		Tags: []string{"Block"},
	}, bh.CreateBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: "/blocks/{block_id}",
		Summary: "Update existing block", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.PatchBlock)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/blocks/{block_id}/{course_id}",
		Summary: "Unlink block from course", DefaultStatus: http.StatusOK,
		Tags: []string{"Block"},
	}, bh.UnlinkFromCourse)

	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: "/blocks/{block_id}",
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
		Method: http.MethodPost, Path: "/courses/invites",
		Summary: "Create invite link for course", DefaultStatus: http.StatusCreated,
		Tags: []string{"Course"},
	}, ch.CreateInvite)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/invites/{invite_id}",
		Summary: "Get invite link details", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.GetInviteDetails)

	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: "/courses/invites/{invite_id}",
		Summary: "Join course by invite link", DefaultStatus: http.StatusOK,
		Tags: []string{"Course"},
	}, ch.JoinCourseByInvite)

	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: "/courses/roles/{course_id}",
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
	}, gh.DeleteSshKey)
}
