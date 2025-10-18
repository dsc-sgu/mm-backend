package main

import (
	"os"
	"strings"

	"github.com/MergeMinds/mm-backend-go/internal/auth/users"
	"github.com/MergeMinds/mm-backend-go/internal/pg"
	"github.com/MergeMinds/mm-backend-go/internal/config"
	"github.com/MergeMinds/mm-backend-go/internal/db"
	"github.com/MergeMinds/mm-backend-go/internal/logger"
	"go.uber.org/zap"
)

func main() {
	config, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}

	logger.Init(config.LogLevel)

	dbConn, err := db.RunSQL(config.Postgres.GetURL(), os.Getenv("SQL_FILE"))
	if err != nil {
		zap.S().Error(err.Error())
		os.Exit(1)
	}

	defer func() {
		if err := dbConn.Close(); err != nil {
			zap.S().Error(err.Error())
		}
	}()

	if strings.ToLower(os.Getenv("CREATE_ADMIN")) == "true" {
		pgRepo := pg.NewPGRepo(dbConn)
		userService := users.NewService(pgRepo)

		_, err = userService.CreateUser(&users.CreateModel{
			FirstName: "Admin",
			LastName:  "Admin",
			Username:  config.Postgres.User,
			Role:      "ADMIN",
			Password:  config.Postgres.Password,
			Email:     "drevniyrus@alivetech.org",
		})
		zap.L().Info("Admin created!")
	}

	if err != nil {
		zap.L().Error(err.Error())
		os.Exit(1)
	}
}
