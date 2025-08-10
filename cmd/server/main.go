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

	api "github.com/MergeMinds/mm-backend-go/internal"
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/config"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/db"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/MergeMinds/mm-backend-go/internal/gitservice"
	"github.com/MergeMinds/mm-backend-go/internal/logger"
	pkggit "github.com/MergeMinds/mm-backend-go/pkg/git"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/rs/cors"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
	"github.com/go-fuego/fuego"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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

	go func() {
		defer wg.Done()
		zap.S().Info("Closing Redis client")
		if err := redisClient.Close(); err != nil {
			zap.S().Warn("Failed closing Redis client: " + err.Error())
			errCh <- err
			// return err
		} else {
			zap.S().Info("Succesfully closed redis client")
		}
	}()

	go func() {
		defer wg.Done()
		zap.S().Info("Closing database connection")
		if err := dbConn.Close(); err != nil {
			zap.S().Warn("Failed closing database connection: " + err.Error())
			errCh <- err
			// return err
		} else {
			zap.S().Info("Succesfully closed database connection")
		}
	}()

	go func() {
		defer wg.Done()
		zap.S().Info("Stopping SSH server")
		if err := app.sshServer.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			errCh <- err
			zap.S().Errorw("Could not stop server", "error", err)
		}
	}()

	go func() {
		defer wg.Done()
		zap.S().Info("Stopping HTTP server")
		if err := app.httpServer.Shutdown(ctx); err != nil {
			errCh <- err
			zap.S().Errorw("Could not stop server", "error", err)
		}
	}()

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

	dbConn, err := db.CreateDb(config.Postgres.GetURL())
	if err != nil {
		zap.S().Fatalf("Unable to establish database connection: %s", err.Error())
	}

	redisOpts, err := redis.ParseURL(config.Redis.GetURL())
	if err != nil {
		zap.S().Fatalf("Unable to establish redis connection: %s", err.Error())
	}
	redisClient := redis.NewClient(redisOpts)

	httpServer := fuego.NewServer(
		fuego.WithAddr(fmt.Sprintf("%s:%d", config.Host, config.HTTPPort)),
		fuego.WithGlobalMiddlewares(logger.ZapMiddleware()),
	)
	httpServer.OpenAPI.Description().Servers = openapi3.Servers{
		{
			URL:         fmt.Sprintf("http://%s:%d", config.OpenAPI.Host, config.OpenAPI.Port),
			Description: "Docker dev deployment server",
		},
	}
	fuego.Use(httpServer, cors.Default().Handler)

	// TODO(nrydanov): Return zap logging middleware
	// https://github.com/MergeMinds/mm-backend-go/issues/26)
	// r.Use(ginzap.Ginzap(zap.S(), time.RFC3339, true))
	// r.Use(ginzap.RecoveryWithZap(zap.S(), true))

	cookieConfig := cookie.DefaultCookieConfig()
	cookieConfig.Secure = config.SessionCookieSecure
	cookieConfig.Domain = config.SessionCookieDomain

	userRepo := user.NewPGRepo(dbConn)
	sessionRepo := session.NewRedisRepo(redisClient)
	blockRepo := blocks.NewPGRepo(dbConn)
	courseRepo := courses.NewPGRepo(dbConn)
	disciplineRepo := disciplines.NewPGRepo(dbConn)

	v1 := fuego.Group(httpServer, "/api/v1")

	api.SetupRoutes(
		v1,
		blockRepo,
		courseRepo,
		disciplineRepo,
		sessionRepo,
		userRepo,
		cookieConfig,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a := gitservice.App{Access: pkggit.ReadWriteAccess}
	sshServer, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(config.Host, strconv.Itoa(config.SSHPort))),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		ssh.PublicKeyAuth(gitservice.CheckPubkeyAuth),
		ssh.PasswordAuth(gitservice.CheckPasswordAuth),
		wish.WithMiddleware(
			pkggit.Middleware("repos", gitservice.RepoRename, a),
			gitservice.GitListMiddleware,
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
	wg.Add(2)

	zap.S().Infof("Starting HTTP server on %s:%d", config.Host, config.HTTPPort)
	go func() {
		defer wg.Done()
		if err := httpServer.Run(); err != nil && err != http.ErrServerClosed {
			zap.S().Error(err.Error())
		}
	}()

	zap.S().Infof("Starting SSH server on %s:%d", config.Host, config.SSHPort)
	go func() {
		defer wg.Done()
		if err := sshServer.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			zap.S().Errorw("Could not start server", "error", err)
		}
	}()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		<-done
	}()

	zap.L().Info("Server is successfully started")

	<-ctx.Done()
	zap.L().Info("Shutting down server. Terminating all active sessions.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.onShutdown(redisClient, dbConn, shutdownCtx); err != nil {
		zap.L().Warn("Failed to shutdown gracefully.")
	} else {
		zap.L().Info("Shutdown complete.")
	}
}
