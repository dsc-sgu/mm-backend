package api

import (
	"net/http"

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
		option.DefaultStatusCode(http.StatusCreated),
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
		option.DefaultStatusCode(http.StatusNoContent),
	)

	courseGroup := fuego.Group(g, "/courses", option.Summary("Course API"))

	fuego.Post(
		courseGroup,
		"/",
		func(m fuego.ContextWithBody[courses.CreateCourse]) (*courses.Course, error) {
			return routes.CreateCourse(sessionRepo, courseRepo, logger, m)
		},
		option.Summary("Create new course"),
		option.DefaultStatusCode(http.StatusCreated),
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
		option.DefaultStatusCode(http.StatusNoContent),
	)

	disciplineGroup := fuego.Group(g, "/disciplines", option.Summary("Discipline API"))

	fuego.Post(
		disciplineGroup,
		"",
		func(m fuego.ContextWithBody[disciplines.CreateDiscipline]) (*disciplines.Discipline, error) {
			return routes.CreateDiscipline(disciplineRepo, logger, m)
		},
		option.Summary("Create new discipline"),
		option.DefaultStatusCode(http.StatusCreated),
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
		option.DefaultStatusCode(http.StatusNoContent),
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
		option.DefaultStatusCode(http.StatusCreated),
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

	fuego.Patch(
		blockGroup,
		"/{block_id}",
		func(m fuego.ContextWithBody[blocks.UpdateBlock]) (*blocks.Block, error) {
			block, err := routes.PatchBlock(blockRepo, m)
			if err != nil {
				return nil, err
			}
			return block, nil
		},
		option.Summary("Update existing block"),
	)

	attemptGroup := fuego.Group(g, "/attempt", option.Summary("Attempt API"))

	fuego.Get(
		attemptGroup,
		"/{id}",
		func(m fuego.ContextNoBody) (*Attempt, error) {
			id := m.PathParam("id")
			attempt, err := routes.GetAttempt(m.Context, id, logger)
			if err != nil {
				return nil, err
			}
			return attempt, nil
		},
		option.Summary("Get attempt by ID"),
	)

	fuego.Post(
		attemptGroup,
		"",
		func(m fuego.ContextWithBody[*Attempt]) (*Attempt, error) {
			attempt, err := routes.CreateAttempt(m.Context, logger)
			if err != nil {
				return nil, err
			}
			return attempt, nil
		},
		option.Summary("Create new attempt"),
	)

	fuego.Patch(
		attemptGroup,
		"/{id}",
		func(m fuego.ContextWithBody[*Attempt]) (*Attempt, error) {
			id := m.Param("id")
			attempt, err := routes.PatchAttempt(m.Context, id, logger)
			if err != nil {
				return nil, err
			}
			return attempt, nil

		},
		option.Summary("Update attempt by ID"),
	)

	fuego.Delete(
		attemptGroup,
		"/{id}",
		func(m fuego.ContextNoBody) (any, error) {
			id := m.PathParam("id")
			attempt, err := routes.DeleteAttempt(m.Context, id, logger)
			if err != nil {
				return nil, err
			}
			return attempt, nil
		},
		option.Summary("Delete attempt by ID"),
	)

}
