package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	api "github.com/MergeMinds/mm-backend-go/internal"
	"github.com/MergeMinds/mm-backend-go/internal/applogger"
	"github.com/MergeMinds/mm-backend-go/internal/auth/cookie"
	"github.com/MergeMinds/mm-backend-go/internal/auth/session"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/blocks"
	"github.com/MergeMinds/mm-backend-go/internal/config"
	"github.com/MergeMinds/mm-backend-go/internal/courses"
	"github.com/MergeMinds/mm-backend-go/internal/db"
	"github.com/MergeMinds/mm-backend-go/internal/disciplines"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/rs/cors"

	"github.com/go-fuego/fuego"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func onShutdown(
	redisClient *redis.Client,
	dbConn *sqlx.DB,
	logger *zap.Logger,
) error {
	logger.Info("Closing Redis client")
	if err := redisClient.Close(); err != nil {
		logger.Warn("Failed closing Redis client: " + err.Error())
		return err
	} else {
		logger.Info("Succesfully closed redis client")
	}

	logger.Info("Closing database connection")
	if err := dbConn.Close(); err != nil {
		logger.Warn("Failed closing database connection: " + err.Error())
		return err
	} else {
		logger.Info("Succesfully closed database connection")
	}
	return nil
}

func main() {
	config, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}

	logger := applogger.Create(config.LogLevel)

	dbConn, err := db.CreateDb(config.PostgresUrl, logger)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	redisOpts, err := redis.ParseURL(config.RedisUrl)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)

	s := fuego.NewServer(
		fuego.WithAddr("0.0.0.0:80"),
	)
	s.OpenAPI.Description().Servers = openapi3.Servers{
		{
			URL:         "http://localhost:8080",
			Description: "Docker dev deployment server",
		},
	}
	fuego.Use(s, cors.Default().Handler)

	// TODO(nrydanov): Return zap logging middleware
	// https://github.com/MergeMinds/mm-backend-go/issues/26)
	// r.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	// r.Use(ginzap.RecoveryWithZap(logger, true))

	cookieConfig := cookie.DefaultCookieConfig()
	cookieConfig.Secure = config.SessionCookieSecure
	cookieConfig.Domain = config.SessionCookieDomain

	userRepo := user.NewPGRepo(dbConn, logger)
	sessionRepo := session.NewRedisRepo(redisClient, logger)
	blockRepo := blocks.NewPGRepo(dbConn, logger)
	courseRepo := courses.NewPGRepo(dbConn, logger)
	disciplineRepo := disciplines.NewPGRepo(dbConn, logger)

	v1 := fuego.Group(s, "/api/v1")

	api.SetupRoutes(
		v1,
		blockRepo,
		courseRepo,
		disciplineRepo,
		sessionRepo,
		userRepo,
		logger,
		cookieConfig,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverShutdown := make(chan struct{})

	go func() {
		if err := s.Run(); err != nil && err != http.ErrServerClosed {
			logger.Error(err.Error())
		}
		close(serverShutdown)
	}()

	<-ctx.Done()
	logger.Info("Shutting down server. Terminating all active sessions.")

	if err := onShutdown(redisClient, dbConn, logger); err != nil {
		logger.Warn("Failed to shutdown gracefully.")
	} else {
		logger.Info("Shutdown gracefully.")
	}
}
