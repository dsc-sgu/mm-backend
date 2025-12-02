package api

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"go.uber.org/zap"

	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/routes"
	"github.com/dsc-sgu/mm-backend/pkg/middleware"
)

func SetupRoutes(
	g *fuego.Server,
	blockService *routes.BlockService,
	courseService *routes.CourseService,
	disciplineService *routes.DisciplineService,
	userService *routes.UserService,
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
	config *config.Config,
	logger *zap.Logger,
) {
	var mws []func(http.Handler) http.Handler
	if config.EnableAuth {
		mws = append(mws, middleware.AuthMiddleware(sessionRepo))
	} else {
		mws = append(mws, middleware.FakeAuthMiddleware())
	}

	var privateGroup *fuego.Server
	if config.EnableAuth {
		privateGroup = fuego.Group(
			g,
			"",
			option.Middleware(mws...),
		)
		zap.L().Info("Authorization is enabled")
	} else {
		privateGroup = fuego.Group(g, "")
		zap.L().Info("Authorization is disabled")
	}

	blockGroup := fuego.Group(
		privateGroup,
		"/blocks",
		option.Summary("Block API"),
	)

	fuego.Get(
		blockGroup,
		"/{block_id}",
		func(ctx fuego.ContextNoBody) (*blocks.Block, error) {
			return blockService.GetBlock(ctx)
		},
		option.Summary("Get block by id"),
	)

	fuego.Post(
		blockGroup,
		"/{course_id}/blocks",
		func(ctx fuego.ContextWithBody[blocks.CreateBlock]) (*blocks.Block, error) {
			return blockService.CreateBlock(ctx)
		},
		option.Summary("Create new block on course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Patch(
		blockGroup,
		"/{block_id}",
		func(ctx fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
			return blockService.PatchBlock(ctx)
		},
		option.Summary("Update existing block"),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}/{course_id}",
		func(ctx fuego.ContextNoBody) (*blocks.Block, error) {
			return blockService.UnlinkFromCourse(ctx)
		},
		option.Summary("Unlink block from course"),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}",
		func(ctx fuego.ContextNoBody) (any, error) {
			return blockService.DeleteBlock(ctx)
		},
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
		"/",
		func(ctx fuego.ContextWithBody[courses.CreateCourse]) (*courses.Course, error) {
			return courseService.CreateCourse(ctx)
		},
		option.Summary("Create new course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Get(
		courseGroup,
		"/{course_id}",
		func(ctx fuego.ContextNoBody) (*courses.Course, error) {
			return courseService.GetCourse(ctx)
		},
		option.Summary("Get existing course by id"),
	)

	fuego.Get(
		courseGroup,
		"",
		func(ctx fuego.ContextNoBody) ([]*courses.Course, error) {
			return courseService.GetPaginatedCourses(ctx)
		},
		option.Summary("Get paginated courses"),
		option.QueryInt("limit", "Number of courses in response"),
		option.QueryInt("offset", "Offset from list beginning"),
	)

	fuego.Patch(
		courseGroup,
		"/{course_id}",
		func(ctx fuego.ContextWithBody[courses.UpdateCourse]) (*courses.Course, error) {
			return courseService.PatchCourse(ctx)
		},
		option.Summary("Update existing course"),
	)

	fuego.Delete(
		courseGroup,
		"/{course_id}",
		func(ctx fuego.ContextNoBody) (any, error) {
			return courseService.DeleteCourse(blockService, ctx)
		},
		option.Summary("Delete course"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	disciplineGroup := fuego.Group(
		privateGroup,
		"/disciplines",
		option.Summary("Discipline API"),
	)

	fuego.Post(
		disciplineGroup,
		"",
		func(ctx fuego.ContextWithBody[disciplines.CreateDiscipline]) (*disciplines.Discipline, error) {
			return disciplineService.CreateDiscipline(ctx)
		},
		option.Summary("Create new discipline"),
		option.DefaultStatusCode(http.StatusCreated),
	)
	fuego.Get(
		disciplineGroup,
		"/{discipline_id}",
		func(ctx fuego.ContextNoBody) (*disciplines.Discipline, error) {
			return disciplineService.GetDiscipline(ctx)
		},
		option.Summary("Get discipline by id"),
	)

	fuego.Patch(
		disciplineGroup,
		"/{discipline_id}",
		func(ctx fuego.ContextWithBody[disciplines.PatchDiscipline]) (*disciplines.Discipline, error) {
			return disciplineService.PatchDiscipline(ctx)
		},
		option.Summary("Update existing discipline"),
	)
	fuego.Delete(
		disciplineGroup,
		"/{discipline_id}",
		func(ctx fuego.ContextNoBody) (any, error) {
			return disciplineService.DeleteDiscipline(ctx)
		},
		option.DefaultStatusCode(http.StatusNoContent),
		option.Summary("Delete discipline"),
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
		func(ctx fuego.ContextWithBody[routes.LoginModel]) (any, error) {
			return userService.Login(sessionRepo, cookieConfig, ctx)
		},
		option.Summary("Login user"),
	)

	fuego.Post(
		authGroup,
		"/register",
		func(ctx fuego.ContextWithBody[routes.RegisterModel]) (any, error) {
			return userService.Register(sessionRepo, cookieConfig, ctx)
		},
		option.Summary("Register new user"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	// Logout
	fuego.Post(
		privateAuthGroup,
		"/logout",
		func(ctx fuego.ContextNoBody) (any, error) {
			return userService.Logout(sessionRepo, cookieConfig, ctx)
		},
		option.Summary("Logout user"),
	)

	// Session
	fuego.Get(
		privateAuthGroup,
		"/session",
		func(ctx fuego.ContextNoBody) (any, error) {
			return userService.GetSession(
				cookieConfig,
				ctx,
			)
		},
	)
}
