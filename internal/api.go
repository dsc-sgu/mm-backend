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
	r *fizz.RouterGroup,
	userRepo user.Repo,
	sessionRepo session.Repo,
	blockRepo blocks.Repo,
	courseRepo courses.Repo,
	disciplineRepo disciplines.Repo,
	logger *zap.Logger,
	cookieConfig *cookie.CookieConfig,
) {
	r.POST("/login", []fizz.OperationOption{
		fizz.Summary("Вход в аккаунт"),
		fizz.Description("Вход пользователя по email и паролю"),
		fizz.Response("401", "Неверные учетные данные", struct{}{}, nil, struct{}{}),
	},
		tonic.Handler(func(c *gin.Context, m *routes.LoginModel) (any, error) {
			return routes.Login(c, userRepo, sessionRepo, logger, cookieConfig, m)
		}, 201))

	// TODO(nrydanov): Add good examples
	r.POST("/register", []fizz.OperationOption{
		fizz.Summary("Регистрация пользователя"),
		fizz.Description("Создание нового пользователя"),
		fizz.Response("400", "Некорректный JSON", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("401", "Неверные учетные данные", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *routes.RegisterModel) (any, error) {
			return routes.Register(c, userRepo, sessionRepo, logger, cookieConfig, m)
		}, 201))

	// // Logout
	r.POST("/logout", []fizz.OperationOption{
		fizz.Summary("Выход из аккаунта"),
		fizz.Description("Завершение сессии пользователя"),
		fizz.Response("401", "Cookie не существует", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *struct{}) (any, error) {
			return routes.Logout(c, userRepo, sessionRepo, logger, cookieConfig)
		}, 201))

	// // Session
	r.GET("/session", []fizz.OperationOption{
		fizz.Summary("Получить активную сессию"),
		fizz.Description("Получение информации о текущем пользователе по сессии"),
		fizz.Response("401", "Пользователь не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Пользователь не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *struct{}) (any, error) {
			routes.Session(c, userRepo, sessionRepo, logger, cookieConfig)
			return nil, nil
		}, 200))

	// // Blocks CRUD
	r.GET("/blocks/:block_id", []fizz.OperationOption{
		fizz.Summary("Получить блок по ID"),
		fizz.Description("Получение данных блока по идентификатору"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.BlockID) (*blocks.BlockType, error) {
			return routes.GetBlock(c, blockRepo, m)
		}, 201))

	r.POST("/blocks", []fizz.OperationOption{
		fizz.Summary("Создать блок"),
		fizz.Description("Создание нового блока"),
		fizz.Response("400", "Некорректный JSON", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("403", "Нет прав", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.CreateBlockType) (*blocks.BlockType, error) {
			return routes.CreateBlock(c, blockRepo, m)
		}, 201))

	r.PATCH("/blocks/:block_id", []fizz.OperationOption{
		fizz.Summary("Изменить блок"),
		fizz.Description("Изменение параметров блока"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Параметр не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.UpdateBlockType) (*blocks.BlockType, error) {
			return routes.PatchBlock(c, blockRepo, m)
		}, 200))

	r.GET("/courses/:course_id/blocks", []fizz.OperationOption{
		fizz.Summary("Получить блоки, связанные с курсом"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Параметр не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) ([]*blocks.BlockType, error) {
			return routes.GetAllBlocks(c, blockRepo, m)
		}, 200))
	r.POST("/courses/:course_id/blocks", []fizz.OperationOption{
		fizz.Summary("Добавить блок в курс"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Параметр не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.CreateBlockType) (*blocks.BlockType, error) {
			return routes.CreateBlock(c, blockRepo, m)
		}, 200))

	r.DELETE("/blocks/:block_id", []fizz.OperationOption{
		fizz.Summary("Удалить блок из курса"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Параметр не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.BlockID) (*blocks.BlockType, error) {
			return routes.DeleteBlock(c, blockRepo, m)
		}, 200))

	r.DELETE("/courses/:course_id/blocks/:block_id", []fizz.OperationOption{
		fizz.Summary("Удалить блок"),
		fizz.Description("Удаление блока из курса (не из базы)"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *blocks.DeleteBlockFromCourse) (*struct{}, error) {
			// return routes.UnlinkBlock(c, blockRepo, m)
			return &struct{}{}, nil
		}, 204))

	r.POST("/courses", []fizz.OperationOption{
		fizz.Summary("Добавить курс"),
		fizz.Description("Удаление блока из курса (не из базы)"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CreateCourseType) (*courses.CourseType, error) {
			return routes.CreateCourse(c, sessionRepo, courseRepo, logger, m)
		}, 204))

	r.GET("/courses/:course_id", []fizz.OperationOption{
		fizz.Summary("Получить курс"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) (*courses.CourseType, error) {
			return routes.GetCourse(c, courseRepo, logger, m)
		}, 204))

	r.GET("/courses/", []fizz.OperationOption{
		fizz.Summary("Получить все курсы"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CoursePagination) ([]*courses.CourseType, error) {
			return routes.GetCourselistPage(c, courseRepo, logger, m)
		}, 204))

	r.PATCH("/courses/:course_id", []fizz.OperationOption{
		fizz.Summary("Изменить курс"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.UpdateCourseType) (*courses.CourseType, error) {
			return routes.PatchCourse(c, courseRepo, logger, m)
		}, 204))

	r.DELETE("/courses/:course_id", []fizz.OperationOption{
		fizz.Summary("Удалить курс"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *courses.CourseID) (*struct{}, error) {
			return routes.DeleteCourse(c, courseRepo, logger, m)
		}, 204))

	r.POST("/disciplines", []fizz.OperationOption{
		fizz.Summary("Создать дисциплину"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный JSON", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("403", "Нет прав", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.CreateDisciplineType) (*disciplines.DisciplineType, error) {
			return routes.CreateDiscipline(c, disciplineRepo, logger, m)
		}, 201))

	r.GET("/disciplines/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Получить дисциплину"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Дисциплина не найдена", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.DisciplineID) (*disciplines.DisciplineType, error) {
			return routes.GetDiscipline(c, disciplineRepo, logger, m)
		}, 200))

	r.PATCH("/disciplines/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Изменить дисциплину"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Дисциплина не найдена", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.CreateDisciplineType) (*disciplines.DisciplineType, error) {
			return routes.PatchDiscipline(c, disciplineRepo, logger, m)
		}, 200))

	r.DELETE("/disciplines/:discipline_id", []fizz.OperationOption{
		fizz.Summary("Удалить дисциплину"),
		fizz.Description(""),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Дисциплина не найдена", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *disciplines.DisciplineID) (*struct{}, error) {
			return routes.DeleteDiscipline(c, disciplineRepo, logger, m)
		}, 204))

}
