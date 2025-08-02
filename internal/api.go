package api

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/MergeMinds/mm-backend-go/internal/routes"
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"go.uber.org/zap"
)

func SetupRoutes(
	g *fuego.Server,
	blockRepo blocks.Repo,
	courseRepo courses.Repo,
	disciplineRepo disciplines.Repo,
	sessionRepo session.Repo,
	userRepo user.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
) {
	blockGroup := fuego.Group(g, "/blocks", option.Summary("Block API"))

	fuego.Get(
		blockGroup,
		"/{block_id}",
		func(m fuego.ContextNoBody) (*blocks.Block, error) {
			return routes.GetBlock(blockRepo, m)
		},
		option.Summary("Get block by id"),
	)

	fuego.Post(
		blockGroup,
		"/{course_id}/blocks",
		func(m fuego.ContextWithBody[blocks.CreateBlock]) (*blocks.Block, error) {
			return routes.CreateBlock(blockRepo, m)
		},
		option.Summary("Create new block on course"),
	)
	fuego.Patch(
		blockGroup,
		"/{block_id}",
		func(m fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
			return routes.PatchBlock(blockRepo, m)
		},
		option.Summary("Update existing block"),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}/{course_id}",
		func(m fuego.ContextNoBody) (*blocks.Block, error) {
			return routes.UnlinkFromCourse(blockRepo, m)
		},
		option.Summary("Unlink block from course"),
	)

	fuego.Delete(
		blockGroup,
		"/{block_id}",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.DeleteBlock(blockRepo, m)
		},
		option.Summary("Delete block from course"),
	)

	courseGroup := fuego.Group(g, "/courses", option.Summary("Course API"))

	fuego.Post(
		courseGroup,
		"/",
		func(m fuego.ContextWithBody[courses.CreateCourse]) (*courses.Course, error) {
			return routes.CreateCourse(sessionRepo, courseRepo, logger, m)
		},
		option.Summary("Create new course"),
	)

	fuego.Get(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextNoBody) (*courses.Course, error) {
			return routes.GetCourse(courseRepo, logger, m)
		},
		option.Summary("Get existing course by id"),
	)

	fuego.Get(
		courseGroup,
		"",
		func(m fuego.ContextNoBody) ([]*courses.Course, error) {
			return routes.GetPaginatedCourses(courseRepo, logger, m)
		},
		option.Summary("Get paginated courses"),
		option.QueryInt("limit", "Number of courses in response"),
		option.QueryInt("offset", "Offset from list beginning"),
	)

	fuego.Patch(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextWithBody[courses.UpdateCourse]) (*courses.Course, error) {
			return routes.PatchCourse(courseRepo, logger, m)
		},
		option.Summary("Update existing course"),
	)

	fuego.Delete(
		courseGroup,
		"/{course_id}",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.DeleteCourse(courseRepo, blockRepo, logger, m)
		},
		option.Summary("Delete course"),
	)

	disciplineGroup := fuego.Group(g, "/disciplines", option.Summary("Discipline API"))

	fuego.Post(
		disciplineGroup,
		"",
		func(m fuego.ContextWithBody[disciplines.CreateDiscipline]) (*disciplines.Discipline, error) {
			return routes.CreateDiscipline(disciplineRepo, logger, m)
		},
		option.Summary("Create new discipline"),
	)
	fuego.Get(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextNoBody) (*disciplines.Discipline, error) {
			return routes.GetDiscipline(disciplineRepo, logger, m)
		},
		option.Summary("Get discipline by id"),
	)

	fuego.Patch(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextWithBody[disciplines.PatchDiscipline]) (*disciplines.Discipline, error) {
			return routes.PatchDiscipline(disciplineRepo, logger, m)
		},
		option.Summary("Update existing discipline"),
	)
	fuego.Delete(
		disciplineGroup,
		"/{discipline_id}",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.DeleteDiscipline(disciplineRepo, logger, m)
		},
		option.Summary("Delete discipline"),
	)

	authGroup := fuego.Group(g, "/auth", option.Summary("Auth API"))

	fuego.Post(
		authGroup,
		"/login",
		func(m fuego.ContextWithBody[routes.LoginModel]) (any, error) {
			return routes.Login(userRepo, sessionRepo, logger, cookieConfig, m)
		},
		option.Summary("Login user"),
	)

	fuego.Post(
		authGroup,
		"/register",
		func(m fuego.ContextWithBody[routes.RegisterModel]) (any, error) {
			return routes.Register(userRepo, sessionRepo, logger, cookieConfig, m)
		},
		option.Summary("Register new user"),
	)

	// Logout
	fuego.Post(
		authGroup,
		"/logout",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.Logout(userRepo, sessionRepo, logger, cookieConfig, m)
		},
		option.Summary("Logout user"),
	)

	// Session
	fuego.Get(
		authGroup,
		"/session",
		func(m fuego.ContextNoBody) (any, error) {
			return routes.Session(userRepo, sessionRepo, logger, cookieConfig, m)
		},
	)
}
