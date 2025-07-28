package main

import (
	"os"
	"strings"

	"github.com/MergeMinds/mm-backend-go/internal/applogger"
	"github.com/MergeMinds/mm-backend-go/internal/auth/user"
	"github.com/MergeMinds/mm-backend-go/internal/config"
	"github.com/MergeMinds/mm-backend-go/internal/db"
)

func main() {
	config, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}

	logger := applogger.Create(config.LogLevel)

	dbConn, err := db.RunSQL(config.PostgresUrl, os.Getenv("SQL_FILE"), logger)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Error(err.Error())
		}
	}()

	if strings.ToLower(os.Getenv("CREATE_ADMIN")) == "true" {
		userRepo := user.NewPGRepo(dbConn, logger)
		_, err = userRepo.Create(&user.CreateModel{
			FirstName: "Admin",
			LastName:  "Admin",
			Username:  config.AdminUsername,
			Role:      "ADMIN",
			Password:  config.AdminPassword,
			Email:     config.AdminEmail,
		})
		logger.Info("Admin created!")
	}

	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
