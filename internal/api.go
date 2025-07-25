package api

import (
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/MergeMinds/mm-backend-go/internal/routes"
	"github.com/MergeMinds/mm-backend-go/internal/swagger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func SetupRoutes(
	r *gin.RouterGroup,
	userRepo user.Repo,
	sessionRepo session.Repo,
	blockRepo blocks.Repo,
	courseRepo courses.Repo,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
) {
	swagger.InitSwagger(r)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.POST("/login", func(ctx *gin.Context) {
		routes.Login(ctx, userRepo, sessionRepo, logger, cookieConfig)
	})

	r.POST("/register", func(ctx *gin.Context) {
		routes.Register(ctx, userRepo, sessionRepo, logger, cookieConfig)
	})

	r.POST("/logout", func(ctx *gin.Context) {
		routes.Logout(ctx, userRepo, sessionRepo, logger, cookieConfig)
	})

	r.GET("/session", func(ctx *gin.Context) {
		routes.Session(ctx, userRepo, sessionRepo, logger, cookieConfig)
	})

	r.GET("/blocks/:id", func(ctx *gin.Context) {
		routes.GetBlock(ctx, blockRepo, logger)
	})

	r.GET("courses/:course_id/blocks", func(ctx *gin.Context) {
		routes.GetAllBlocks(ctx, blockRepo, logger)
	})
	r.POST("courses/:course_id/blocks", func(ctx *gin.Context) {
		routes.CreateBlock(ctx, blockRepo, logger)
	})

	r.PATCH("/blocks/:id", func(ctx *gin.Context) {
		routes.PatchBlock(ctx, blockRepo, logger)
	})

	r.DELETE("/courses/:course_id/blocks/:id", func(ctx *gin.Context) {
		routes.UnlinkBlock(ctx, blockRepo, logger)
	})

	r.DELETE("/blocks/:id", func(ctx *gin.Context) {
		routes.DeleteBlock(ctx, blockRepo, logger)
	})

	r.POST("/courses", func(ctx *gin.Context) {
		routes.CreateCourse(ctx, sessionRepo, courseRepo, logger)
	})

	r.GET("/courses/:course_id", func(ctx *gin.Context) {
		routes.GetCourse(ctx, courseRepo, logger)
	})

	r.GET("/courses", func(ctx *gin.Context) {
		routes.GetCourselistPage(ctx, courseRepo, logger)
	})

	r.PATCH("/courses/:course_id", func(ctx *gin.Context) {
		routes.PatchCourse(ctx, courseRepo, logger)
	})

	r.DELETE("/courses/:course_id", func(ctx *gin.Context) {
		routes.DeleteCourse(ctx, courseRepo, blockRepo, logger)
	})

	r.POST("/disciplines", func(ctx *gin.Context) {
		routes.CreateDiscipline(ctx, disciplineRepo, logger)
	})

	r.GET("/disciplines/:id", func(ctx *gin.Context) {
		routes.GetDiscipline(ctx, disciplineRepo, logger)
	})

	r.PATCH("/disciplines/:id", func(ctx *gin.Context) {
		routes.PatchDiscipline(ctx, disciplineRepo, logger)
	})
	r.DELETE("/disciplines/:id", func(ctx *gin.Context) {
		routes.DeleteDiscipline(ctx, disciplineRepo, logger)
	})

}
