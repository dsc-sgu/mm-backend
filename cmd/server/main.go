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
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"go.uber.org/zap"

	api "github.com/dsc-sgu/mm-backend/internal"
	"github.com/dsc-sgu/mm-backend/internal/auth/cookie"
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/config"
	"github.com/dsc-sgu/mm-backend/internal/gitservice"
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

	var dbConn *sqlx.DB
	if err != nil {
		zap.S().
			Fatalf("Unable to establish database connection: %s", err.Error())
	}

	// redisOpts, err := redis.ParseURL(config.Redis.GetURL())
	if err != nil {
		zap.S().Fatalf("Unable to establish redis connection: %s", err.Error())
	}
	var redisClient *redis.Client
	// redisClient := redis.NewClient(redisOpts)

	httpServer := fuego.NewServer(
		fuego.WithAddr(fmt.Sprintf("%s:%d", config.Host, config.HTTPPort)),
		fuego.WithGlobalMiddlewares(logger.ZapMiddleware()),
	)
	httpServer.OpenAPI.Description().Servers = openapi3.Servers{
		{
			URL: fmt.Sprintf(
				"http://%s:%d",
				config.OpenAPI.Host,
				config.OpenAPI.Port,
			),
			Description: "Docker dev deployment server",
		},
	}

	corsMiddleware := cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   config.AllowOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
		},
	})

	fuego.Use(httpServer, corsMiddleware.Handler)

	cookieConfig := cookie.DefaultCookieConfig()
	cookieConfig.Secure = config.SessionCookieSecure
	cookieConfig.Domain = config.SessionCookieDomain

	pgRepo := pg.NewPGRepo(dbConn)

	userService := routes.NewUserService(pgRepo)
	blockService := routes.NewBlockService(pgRepo)
	courseService := routes.NewCourseService(pgRepo)
	disciplineService := routes.NewDisciplineService(pgRepo)
	sessionRepo := session.NewRedisRepo(redisClient)

	v1 := fuego.Group(httpServer, "/api/v1")

	api.SetupRoutes(
		v1,
		blockService,
		courseService,
		disciplineService,
		userService,
		sessionRepo,
		cookieConfig,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	a := gitservice.App{Access: pkggit.ReadWriteAccess}
	sshServer, err := wish.NewServer(
		wish.WithAddress(
			net.JoinHostPort(config.Host, strconv.Itoa(config.SSHPort)),
		),
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
