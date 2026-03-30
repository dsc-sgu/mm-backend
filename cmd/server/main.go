package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
	"github.com/go-fuego/fuego"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"go.uber.org/zap"

	api "github.com/dsc-sgu/mm-backend/internal"
	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/lock"
	"github.com/dsc-sgu/mm-backend/internal/db"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/dsc-sgu/mm-backend/internal/logger"
	"github.com/dsc-sgu/mm-backend/internal/pg"
	"github.com/dsc-sgu/mm-backend/internal/routes"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type App struct {
	redis      *redis.Client
	database   *sqlx.DB
	httpServer *fuego.Server
	sshServer  *ssh.Server
}

func (app *App) onShutdown(
	redisClient *redis.Client,
	dbConn *sqlx.DB,
	ctx context.Context,
) error {
	wg := sync.WaitGroup{}
	wg.Add(4)

	errCh := make(chan error)

	wg.Go(func() {
		zap.S().Info("Closing DB connection")
		if err := dbConn.Close(); err != nil {
			zap.S().Warn("Failed closing DB connection: " + err.Error())
			errCh <- err
		} else {
			zap.S().Info("Succesfully closed DB connection")
		}
	})

	wg.Go(func() {
		zap.S().Info("Closing Redis client")
		if err := redisClient.Close(); err != nil {
			zap.S().Warn("Failed closing Redis client: " + err.Error())
			errCh <- err
		} else {
			zap.S().Info("Succesfully closed redis client")
		}
	})

	wg.Go(func() {
		zap.S().Info("Closing SSH server")
		if err := app.sshServer.Shutdown(ctx); err != nil &&
			!errors.Is(err, ssh.ErrServerClosed) {
			errCh <- err
			zap.S().Errorw("Could not stop server", "error", err)
		}
	})

	wg.Go(func() {
		zap.S().Info("Closing HTTP server")
		if err := app.httpServer.Shutdown(ctx); err != nil {
			errCh <- err
			zap.S().Errorw("Could not stop server", "error", err)
		}
	})

	go func() {
		defer close(errCh)
		wg.Wait()
	}()

	errs := make([]error, 0)

	for err := range errCh {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func main() {
	config, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}

	conf := zap.NewDevelopmentConfig()

	conf.Level = config.LogLevel

	zap.ReplaceGlobals(zap.Must(conf.Build()))

	dbConn, err := db.CreateDB(config.Postgres.GetURL())
	if err != nil {
		zap.S().
			Fatalf("establishing database connection: %s", err.Error())
	} else {
		zap.L().Info("Database connection is established")
	}

	redisOpts, err := redis.ParseURL(config.Redis.GetURL())
	if err != nil {
		zap.S().Fatalf("parsing redis url: %s", err.Error())
	}
	redisClient := redis.NewClient(redisOpts)
	zap.L().Info("Redis connection is established")

	corsMiddleware := cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   config.AllowOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
		},
	})

	httpServer := fuego.NewServer(
		fuego.WithAddr(fmt.Sprintf("%s:%d", config.Host, config.HTTPPort)),
		fuego.WithGlobalMiddlewares(
			corsMiddleware.Handler,
			logger.ZapMiddleware(),
		),
	)

	cookieConfig := cookie.DefaultCookieConfig()
	cookieConfig.Secure = config.SessionCookieSecure
	cookieConfig.Domain = config.SessionCookieDomain

	pgRepo := pg.NewPGRepo(dbConn)

	sessionRepo := session.NewRedisRepo(redisClient)
	lockManager := lock.NewRedisManager(redisClient)
	blockService := blocks.NewService(pgRepo)
	courseService := courses.NewService(pgRepo)
	disciplineService := disciplines.NewService(pgRepo)
	userService := users.NewService(pgRepo, sessionRepo, cookieConfig)
	gitService := git.NewService(pgRepo)

	// Controller initialization
	userController := routes.NewUserController(userService)
	blockController := routes.NewBlockController(blockService)
	courseController := routes.NewCourseController(
		courseService,
		blockService,
		lockManager,
	)
	disciplineController := routes.NewDisciplineController(disciplineService)
	gitController := routes.NewGitController(gitService)

	v1 := fuego.Group(httpServer, "/api/v1")

	api.SetupRoutes(
		v1,
		blockController,
		courseController,
		disciplineController,
		userController,
		gitController,
		sessionRepo,
		config,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	a := git.App{Access: pkggit.ReadWriteAccess}
	sshServer, err := wish.NewServer(
		wish.WithAddress(
			net.JoinHostPort(config.Host, strconv.Itoa(config.SSHPort)),
		),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		ssh.PublicKeyAuth(git.CheckPubkeyAuth),
		ssh.PasswordAuth(git.CheckPasswordAuth),
		wish.WithMiddleware(
			pkggit.Middleware("repos", git.RepoRename, a),
			git.GitListMiddleware,
			logging.Middleware(),
		),
	)
	if err != nil {
		zap.S().Errorw("Could not create server", "error", err)
	}

	app := App{
		redis:      redisClient,
		database:   dbConn,
		httpServer: httpServer,
		sshServer:  sshServer,
	}

	wg := sync.WaitGroup{}

	zap.S().Infof("Starting HTTP server on %s:%d", config.Host, config.HTTPPort)
	wg.Go(func() {
		if err := httpServer.Run(); err != nil && err != http.ErrServerClosed {
			zap.S().Error(err.Error())
		}
	})
	zap.S().Infof("Starting SSH server on %s:%d", config.Host, config.SSHPort)
	wg.Go(func() {
		if err := sshServer.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			zap.S().Errorw("Could not start server", "error", err)
		}
	})

	done := make(chan struct{})

	go func() {
		wg.Wait()
		<-done
	}()

	zap.L().Info("Server is successfully started")

	<-ctx.Done()
	zap.L().Info("Shutting down server. Terminating all active sessions.")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := app.onShutdown(redisClient, dbConn, shutdownCtx); err != nil {
		zap.L().Error("Failed to shutdown gracefully.", zap.Error(err))
	} else {
		zap.L().Info("Shutdown complete.")
	}
}
