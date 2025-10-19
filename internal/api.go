package api

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"go.uber.org/zap"

	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/MergeMinds/mm-backend-go/internal/routes"
)

func SetupRoutes(
	g *fuego.Server,
	blockService *routes.BlockService,
	courseService *routes.CourseService,
	disciplineService *routes.DisciplineService,
	userService *routes.UserService,
	sessionRepo session.Repo,
	cookieConfig *cookie.CookieConfig,
) {
	blockGroup := fuego.Group(g, "/blocks", option.Summary("Block API"))

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

	courseGroup := fuego.Group(g, "/courses", option.Summary("Course API"))

	fuego.Post(
		courseGroup,
		"/",
		func(m fuego.ContextWithBody[courses.CreateCourse]) (*courses.Course, error) {
			return routes.CreateCourse(sessionRepo, courseService, zap.L(), m)
		},
		option.Summary("Create new course"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	fuego.Get(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextNoBody) (*courses.Course, error) {
			return routes.GetCourse(courseService, zap.L(), m)
		},
		option.Summary("Get existing course by id"),
	)

	fuego.Get(
		courseGroup,
		"",
		func(m fuego.ContextNoBody) ([]*courses.Course, error) {
			return routes.GetPaginatedCourses(courseService, zap.L(), m)
		},
		option.Summary("Get paginated courses"),
		option.QueryInt("limit", "Number of courses in response"),
		option.QueryInt("offset", "Offset from list beginning"),
	)

	fuego.Patch(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextWithBody[courses.UpdateCourse]) (*courses.Course, error) {
			return routes.PatchCourse(courseService, zap.L(), m)
		},
		option.Summary("Update existing course"),
	)

	fuego.Delete(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.DeleteCourse(courseService, blockService, zap.L(), m)
		},
		option.Summary("Delete course"),
		option.DefaultStatusCode(http.StatusNoContent),
	)

	disciplineGroup := fuego.Group(g, "/disciplines", option.Summary("Discipline API"))

	fuego.Post(
		disciplineGroup,
		"",
		func(m fuego.ContextWithBody[disciplines.CreateDiscipline]) (*disciplines.Discipline, error) {
			return routes.CreateDiscipline(disciplineService, zap.L(), m)
		},
		option.Summary("Create new discipline"),
		option.DefaultStatusCode(http.StatusCreated),
	)
	fuego.Get(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextNoBody) (*disciplines.Discipline, error) {
			return routes.GetDiscipline(disciplineService, zap.L(), m)
		},
		option.Summary("Get discipline by id"),
	)

	fuego.Patch(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextWithBody[disciplines.PatchDiscipline]) (*disciplines.Discipline, error) {
			return routes.PatchDiscipline(disciplineService, zap.L(), m)
		},
		option.Summary("Update existing discipline"),
	)
	fuego.Delete(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.DeleteDiscipline(disciplineService, zap.L(), m)
		},
		option.DefaultStatusCode(http.StatusNoContent),
		option.Summary("Delete discipline"),
	)

	authGroup := fuego.Group(g, "/auth", option.Summary("Auth API"))

	fuego.Post(
		authGroup,
		"/login",
		func(m fuego.ContextWithBody[routes.LoginModel]) (any, error) {
			return routes.Login(userService, sessionRepo, zap.L(), cookieConfig, m)
		},
		option.Summary("Login user"),
	)

	fuego.Post(
		authGroup,
		"/register",
		func(m fuego.ContextWithBody[routes.RegisterModel]) (any, error) {
			return routes.Register(userService, sessionRepo, zap.L(), cookieConfig, m)
		},
		option.Summary("Register new user"),
		option.DefaultStatusCode(http.StatusCreated),
	)

	// Logout
	fuego.Post(
		authGroup,
		"/logout",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.Logout(userService, sessionRepo, zap.L(), cookieConfig, m)
		},
		option.Summary("Logout user"),
	)

	// Session
	fuego.Get(
		authGroup,
		"/session",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.Session(userService, sessionRepo, zap.L(), cookieConfig, m)
		},
	)
}
