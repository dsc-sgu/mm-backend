package api

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/dsc-sgu/mm-backend/internal/routes"
	"github.com/dsc-sgu/mm-backend/pkg/middleware"
)

func SetupRoutes(
	g *fuego.Server,
	attemptController *routes.AttemptController,
	blockController *routes.BlockController,
	courseController *routes.CourseController,
	disciplineController *routes.DisciplineController,
	userController *routes.UserController,
	gitController *routes.GitController,
	sessionRepo session.Repo,
	config *config.Config,
) {
	var mws []func(http.Handler) http.Handler
	if config.EnableAuth {
		mws = append(mws, middleware.AuthMiddleware(sessionRepo))
		zap.L().Info("Authorization is enabled")
	} else {
		mws = append(mws, middleware.FakeAuthMiddleware())
		zap.L().Info("Authorization is disabled")
	}

	privateGroup := fuego.Group(
		g,
		"",
		option.Middleware(mws...),
	)

	attemptGroup := fuego.Group(
		privateGroup,
		"/attempts",
		option.Summary("Attempt API"),
	)

	fuego.Get(
		attemptGroup,
		"/diff",
		attemptController.GetDiff,
		option.Summary("Get attempt diff by two id"),
		option.QueryInt("id1", "First attempt ID"),
		option.QueryInt("id2", "Second attempt ID"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Get(
		attemptGroup,
		"/{task_id}/{participant_id}",
		attemptController.GetAttempts,
		option.Summary("Get all user attempts on task"),
		option.DefaultStatusCode(http.StatusOK),
	)

	blockGroup := fuego.Group(
		privateGroup,
		"/blocks",
		option.Summary("Block API"),
	)

	fuego.Get(
		blockGroup,
		"/{block_id}",
		blockController.GetBlock,
		option.Summary("Get block by id"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Post(
		blockGroup,
		"/{course_id}/blocks",
		blockController.CreateBlock,
		option.Summary("Create new block on course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Patch(
		blockGroup,
		"/{block_id}",
		blockController.PatchBlock,
		option.Summary("Update existing block"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}/{course_id}",
		blockController.UnlinkFromCourse,
		option.Summary("Unlink block from course"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}",
		blockController.DeleteBlock,
		option.Summary("Delete block from course"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	courseGroup := fuego.Group(
		privateGroup,
		"/courses",
		option.Summary("Course API"),
	)

	fuego.Post(
		courseGroup,
		"",
		courseController.CreateCourse,
		option.Summary("Create new course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Get(
		courseGroup,
		"/{course_id}",
		courseController.GetCourse,
		option.Summary("Get existing course by id"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Get(
		courseGroup,
		"",
		courseController.GetPaginatedCourses,
		option.Summary("Get paginated courses"),
		option.QueryInt("limit", "Number of courses in response"),
		option.QueryInt("last_id", "Last ID from previous pagination request"),
		option.Query("discipline_id", "An attribute to filet courses by their discipline"),
		option.QueryBool("is_teacher", "An attribute to filter courses by your role"),
		option.QueryBool("is_student", "An attribute to filter courses by your role"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Patch(
		courseGroup,
		"/{course_id}",
		func(ctx fuego.ContextWithBody[courses.UpdateCourse]) (*courses.Course, error) {
			return courseController.PatchCourse(ctx)
		},
		option.Summary("Update existing course"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Delete(
		courseGroup,
		"/{course_id}",
		courseController.DeleteCourse,
		option.Summary("Delete course"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	fuego.Post(
		courseGroup,
		"/invites",
		courseController.CreateInvite,
		option.Summary("Create invite link for course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Get(
		courseGroup,
		"/invites/{invite_id}",
		courseController.GetInviteDetails,
		option.Summary("Get invite link details"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Post(
		courseGroup,
		"/invites/{invite_id}",
		courseController.JoinCourseByInvite,
		option.Summary("Join course by invite link"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Get(
		courseGroup,
		"/roles/{course_id}",
		courseController.GetUserRoleInCourse,
		option.Summary("Get role of current user in course"),
		option.DefaultStatusCode(http.StatusOK),
	)

	disciplineGroup := fuego.Group(
		privateGroup,
		"/disciplines",
		option.Summary("Discipline API"),
	)

	fuego.Post(
		disciplineGroup,
		"",
		disciplineController.CreateDiscipline,
		option.Summary("Create new discipline"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Get(
		disciplineGroup,
		"/{discipline_id}",
		disciplineController.GetDiscipline,
		option.Summary("Get discipline by id"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Patch(
		disciplineGroup,
		"/{discipline_id}",
		disciplineController.PatchDiscipline,
		option.Summary("Update existing discipline"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Delete(
		disciplineGroup,
		"/{discipline_id}",
		disciplineController.DeleteDiscipline,
		option.Summary("Delete discipline"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	gitGroup := fuego.Group(
		privateGroup,
		"/git",
		option.Summary("Git API"),
	)

	fuego.Post(
		gitGroup,
		"/add_key",
		func(ctx fuego.ContextWithBody[git.AddSSHKey]) (any, error) {
			return gitController.AddSSHKey(ctx)
		},
		option.Summary("Add new SSH key"),
		option.DefaultStatusCode(http.StatusAccepted),
	)

	fuego.Delete(
		gitGroup,
		"/delete_key",
		func(ctx fuego.ContextWithBody[git.DeleteSSHKey]) (any, error) {
			return gitController.DeleteSSHKey(ctx)
		},
		option.Summary("Delete SSH key"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	authGroup := fuego.Group(g, "/auth", option.Summary("Auth API"))

	privateAuthGroup := fuego.Group(
		privateGroup,
		"/auth",
		option.Summary("Auth API"),
		option.Middleware(mws...),
	)

	fuego.Post(
		authGroup,
		"/login",
		userController.Login,
		option.Summary("Login user"),
		option.DefaultStatusCode(http.StatusOK),
	)

	fuego.Post(
		authGroup,
		"/register",
		userController.Register,
		option.Summary("Register new user"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	// Logout
	fuego.Post(
		privateAuthGroup,
		"/logout",
		userController.Logout,
		option.Summary("Logout user"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	// Session
	fuego.Get(
		privateAuthGroup,
		"/session",
		userController.GetSession,
		option.Summary("Get session"),
		option.DefaultStatusCode(http.StatusOK),
	)
}
