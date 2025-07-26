package api

import (
	"github.com/MergeMinds/mm-backend-go/internal/apierr"
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/routes"
	"github.com/MergeMinds/mm-backend-go/internal/routes/dto"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"go.uber.org/zap"
)

func SetupRoutes(
	r *fizz.RouterGroup,
	userRepo user.Repo,
	sessionRepo session.Repo,
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
	r.GET("/blocks/:id", []fizz.OperationOption{
		fizz.Summary("Получить блок по ID"),
		fizz.Description("Получение данных блока по идентификатору"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *dto.IdBlockModel) (*dto.BlockType, error) {
			return routes.GetBlock(c, m)
		}, 201))

	r.POST("/blocks", []fizz.OperationOption{
		fizz.Summary("Создать блок"),
		fizz.Description("Создание нового блока"),
		fizz.Response("400", "Некорректный JSON", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("403", "Нет прав", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *dto.CreateBlockType) (*dto.BlockType, error) {
			return routes.CreateBlock(c, m)
		}, 201))

	r.PATCH("/blocks/:id", []fizz.OperationOption{
		fizz.Summary("Изменить блок"),
		fizz.Description("Изменение параметров блока"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Параметр не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *dto.CreateBlockType) (*dto.BlockType, error) {
			return routes.PatchBlock(c, m)
		}, 200))

	r.DELETE("/blocks/:id", []fizz.OperationOption{
		fizz.Summary("Удалить блок"),
		fizz.Description("Удаление блока из курса (не из базы)"),
		fizz.Response("400", "Некорректный ID", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("404", "Блок не найден", apierr.ApiError{}, nil, apierr.ApiError{}),
		fizz.Response("500", "Внутренняя ошибка сервера", apierr.ApiError{}, nil, apierr.ApiError{}),
	},
		tonic.Handler(func(c *gin.Context, m *dto.IdBlockModel) (*struct{}, error) {
			return routes.DeleteBlock(c, m)
		}, 204))
}
