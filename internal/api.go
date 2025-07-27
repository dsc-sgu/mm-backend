package api

import (
	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/MergeMinds/mm-backend-go/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"go.uber.org/zap"
)

func SetupRoutes(
	g *fizz.RouterGroup,
	blockRepo blocks.Repo,
	courseRepo courses.Repo,
	disciplineRepo disciplines.Repo,
	sessionRepo session.Repo,
	userRepo user.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
) {
	blockGroup := g.Group("/blocks", "Block API", "")
	// // Blocks CRUD
	blockGroup.GET("/:block_id", []fizz.OperationOption{
		fizz.Summary("Get block by ID"),
		fizz.Response("404", "Block not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.BlockID) (*blocks.BlockType, error) {
			return routes.GetBlock(c, blockRepo, m)
		}, 201))

	blockGroup.POST("", []fizz.OperationOption{
		fizz.Summary("Create new block"),
		fizz.Response("403", "Not authorized", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.CreateBlockType) (*blocks.BlockType, error) {
			return routes.CreateBlock(c, blockRepo, m)
		}, 201))

	blockGroup.PATCH("/:block_id", []fizz.OperationOption{
		fizz.Summary("Edit existing block"),
		fizz.Response("404", "Block not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.UpdateBlockType) (*blocks.BlockType, error) {
			return routes.PatchBlock(c, blockRepo, m)
		}, 201))

	blockGroup.DELETE("/:block_id", []fizz.OperationOption{
		fizz.Summary("Delete existing block"),
		fizz.Response("404", "Block not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.BlockID) (*struct{}, error) {
			return routes.DeleteBlock(c, blockRepo, m)
		}, 200))

	blockGroup.POST("/:course_id/blocks", []fizz.OperationOption{
		fizz.Summary("Add new block to course"),
		fizz.Response("404", "Course not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.CreateBlockType) (*blocks.BlockType, error) {
			return routes.CreateBlock(c, blockRepo, m)
		}, 201))

	courseGroup := g.Group("/courses", "Course API", "")

	courseGroup.POST("", []fizz.OperationOption{
		fizz.Summary("Create new course"),
		fizz.Response("403", "Not authorized", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CreateCourseType) (*courses.CourseType, error) {
			return routes.CreateCourse(c, sessionRepo, courseRepo, logger, m)
		}, 201))

	courseGroup.GET("/:course_id", []fizz.OperationOption{
		fizz.Summary("Get course data by ID"),
		fizz.Response("404", "Course not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) (*courses.CourseType, error) {
			return routes.GetCourse(c, courseRepo, logger, m)
		}, 200))

	courseGroup.GET("", []fizz.OperationOption{
		fizz.Summary("Get paginated list of courses"),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CoursePagination) ([]*courses.CourseType, error) {
			return routes.GetCourselistPage(c, courseRepo, logger, m)
		}, 200))

	courseGroup.PATCH("/:course_id", []fizz.OperationOption{
		fizz.Summary("Update existing course data"),
		fizz.Response("404", "Course not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.UpdateCourseType) (*courses.CourseType, error) {
			return routes.PatchCourse(c, courseRepo, logger, m)
		}, 204))

	courseGroup.DELETE("/:course_id", []fizz.OperationOption{
		fizz.Summary("Delete existing course"),
		fizz.Response("404", "Course not found", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("403", "Not authorized", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) (*struct{}, error) {
			return routes.DeleteCourse(c, courseRepo, blockRepo, logger, m)
		}, 204))

	courseGroup.GET("/:course_id/blocks", []fizz.OperationOption{
		fizz.Summary("Get all blocks related to course"),
		fizz.Response("404", "Course not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) ([]*blocks.BlockType, error) {
			return routes.GetAllBlocks(c, blockRepo, m)
		}, 200))
	disciplineGroup := g.Group("/disciplines", "Discipline API", "")

	disciplineGroup.POST("", []fizz.OperationOption{
		fizz.Summary("Create new discipline"),
		fizz.Response("403", "Not authorized", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.CreateDisciplineType) (*disciplines.DisciplineType, error) {
			return routes.CreateDiscipline(c, disciplineRepo, logger, m)
		}, 201))

	disciplineGroup.GET("/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Get discipline data by ID"),
		fizz.Response("404", "Discipline not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.DisciplineID) (*disciplines.DisciplineType, error) {
			return routes.GetDiscipline(c, disciplineRepo, logger, m)
		}, 200))

	disciplineGroup.PATCH("/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Update existing discipline data"),
		fizz.Response("404", "Discipline not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.CreateDisciplineType) (*disciplines.DisciplineType, error) {
			return routes.PatchDiscipline(c, disciplineRepo, logger, m)
		}, 201))

	disciplineGroup.DELETE("/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Delete existing discipline"),
		fizz.Response("404", "Discipline not found", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("403", "Not authorized", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.DisciplineID) (*struct{}, error) {
			return routes.DeleteDiscipline(c, disciplineRepo, logger, m)
		}, 204))

	authGroup := g.Group("/login", "Auth API", "")

	authGroup.POST("/login", []fizz.OperationOption{
		fizz.Summary("User login"),
		fizz.Response("401", "Wrong user credentials", struct{}{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *routes.LoginModel) (any, error) {
			return routes.Login(c, userRepo, sessionRepo, logger, cookieConfig, m)
		}, 201))

	// TODO(nrydanov): Add good examples
	authGroup.POST("/register", []fizz.OperationOption{
		fizz.Summary("User registration"),
		fizz.Response("401", "User already exists", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *routes.RegisterModel) (any, error) {
			return routes.Register(c, userRepo, sessionRepo, logger, cookieConfig, m)
		}, 201))

	// // Logout
	authGroup.POST("/logout", []fizz.OperationOption{
		fizz.Summary("User logout"),
		fizz.Response("401", "Cookie not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *struct{}) (any, error) {
			return routes.Logout(c, userRepo, sessionRepo, logger, cookieConfig)
		}, 201))

	// // Session
	authGroup.GET("/session", []fizz.OperationOption{
		fizz.Summary("Get active session"),
		fizz.Response("401", "User not found", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "User not found", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *struct{}) (any, error) {
			routes.Session(c, userRepo, sessionRepo, logger, cookieConfig)
			return nil, nil
		}, 200))
}
