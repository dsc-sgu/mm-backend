package db

import (
	"context"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func RunSQL(dbURL string, filePath string) (*sqlx.DB, error) {
	db, err := CreateDB(dbURL)
	if err != nil {
		zap.L().Error(err.Error())
		os.Exit(1)
	}

	createTableSQL, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(context.Background(), string(createTableSQL))
	if err != nil {
		return nil, err
	}

	zap.L().Info("SQL is done!")

	return db, err
}

func CreateDB(dbURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		zap.S().Errorf("Unable to establish database connection: %s", err.Error())
		os.Exit(1)
	}

	return db, err
}
