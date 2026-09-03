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
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"go.uber.org/zap"

	api "github.com/dsc-sgu/mm-backend/internal"
	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/sshkeys"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/content"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/locks"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/db"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
	"github.com/dsc-sgu/mm-backend/internal/logger"
	"github.com/dsc-sgu/mm-backend/internal/pg"
	"github.com/dsc-sgu/mm-backend/internal/snapshots"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type App struct {
	redis      *redis.Client
	database   *sqlx.DB
	httpServer *http.Server
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
		zap.S().Fatalf("establishing database connection: %s", err.Error())
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

	mux := http.NewServeMux()

	sessionRepo := session.NewRedisRepo(redisClient)

	humaConfig := huma.DefaultConfig(config.OpenAPI.Title, "1.0.0")
	humaConfig.Info.Description = config.OpenAPI.Description
	humaConfig.DocsPath = "/docs"
	humaConfig.DocsRenderer = huma.DocsRendererScalar

	humaAPI := humago.New(mux, humaConfig)
	v1 := huma.NewGroup(humaAPI, "/api/v1")

	cookieConfig := cookie.DefaultCookieConfig()
	cookieConfig.Secure = config.SessionCookieSecure
	cookieConfig.Domain = config.SessionCookieDomain

	pgRepo := pg.NewPGRepo(dbConn)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// Initialize services (the order is important)
	lockService := locks.NewService(pgRepo, config.CourseLockTTLSeconds)
	snapshotService := snapshots.NewService(pgRepo)
	membershipService := membership.NewService(pgRepo)
	userService := users.NewService(pgRepo, sessionRepo, cookieConfig)
	disciplineService := disciplines.NewService(pgRepo)
	gitManager := pkggit.NewManager("repos", config.Host, strconv.Itoa(config.SSHPort))
	sshKeyService := sshkeys.NewService(pgRepo)
	taskService := tasks.NewService(pgRepo, gitManager)
	attemptService := attempt.NewService(pgRepo, gitManager, taskService, pgRepo, sshKeyService)

	rebalanceWorker := blocks.NewRebalanceWorker(pgRepo, 64)
	go rebalanceWorker.Run(ctx)

	blockService := blocks.NewService(pgRepo, rebalanceWorker, config.LexoRankThreshold)
	courseService := courses.NewService(pgRepo, snapshotService, lockService, blockService, membershipService, userService)

	userHandler := users.NewHandler(userService)
	blockHandler := blocks.NewHandler(blockService)
	contentHandler := content.NewHandler(content.NewService(pgRepo, rebalanceWorker, config.LexoRankThreshold))
	courseHandler := courses.NewHandler(courseService, lockService, membershipService, userService)
	disciplineHandler := disciplines.NewHandler(disciplineService)
	sshKeyHandler := sshkeys.NewHandler(sshKeyService)
	taskHandler := tasks.NewHandler(taskService)
	attemptHandler := attempt.NewHandler(attemptService)

	api.SetupRoutes(
		v1,
		blockHandler,
		contentHandler,
		courseHandler,
		disciplineHandler,
		userHandler,
		sshKeyHandler,
		attemptHandler,
		taskHandler,
		sessionRepo,
		config,
	)

	var handler http.Handler = mux
	handler = logger.ZapMiddleware()(handler)
	handler = corsMiddleware.Handler(handler)

	addr := fmt.Sprintf("%s:%d", config.Host, config.HTTPPort)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	sshServer, err := wish.NewServer(
		wish.WithAddress(
			net.JoinHostPort(config.Host, strconv.Itoa(config.SSHPort)),
		),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		ssh.PublicKeyAuth(sshKeyService.CheckPublicKeyAuth),
		ssh.PasswordAuth(sshKeyService.CheckPasswordAuth),
		wish.WithMiddleware(
			attemptService.SSHMiddleware("repos"),
			gitManager.ListMiddleware,
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

	zap.S().Infof("Starting HTTP server on %s", addr)
	wg.Go(func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zap.S().Error(err.Error())
		}
	})

	zap.S().Infof("Starting SSH server on %s:%d", config.Host, config.SSHPort)
	wg.Go(func() {
		if err := sshServer.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			zap.S().Errorw("Could not start server", "error", err)
		}
	})

	zap.L().Info("Server running")
	<-ctx.Done()
	zap.L().Info("Shutting down server. Terminating all active sessions.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.onShutdown(redisClient, dbConn, shutdownCtx); err != nil {
		zap.L().Error("Failed to shutdown gracefully.", zap.Error(err))
	} else {
		zap.L().Info("Shutdown complete.")
	}
}
