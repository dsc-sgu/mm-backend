package db

import (
	"context"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func RunSQL(dbUrl string, filePath string) (*sqlx.DB, error) {
	db, err := CreateDb(dbUrl)
	if err != nil {
		zap.L().Error(err.Error())
		os.Exit(1)
	}

	createTableSql, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(context.Background(), string(createTableSql))
	if err != nil {
		return nil, err
	}

	zap.L().Info("SQL is done!")

	return db, err
}

func CreateDb(dbUrl string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", dbUrl)
	if err != nil {
		zap.S().Errorf("Unable to establish database connection: %s", err.Error())
		os.Exit(1)
	}

	return db, err
}
