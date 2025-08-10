# Run the application in Docker Compose without initializing the database. This requires manual initialization via 'just initdb-compose'
run-deps:
    docker compose up db dragonfly -d

# Run the application in Docker Compose with Air (for development)
dev:
    docker compose up backend-dev db dragonfly

# Initialize the database by running the Go executable on the host system
initdb:
    SQL_FILE=./db/CreateTables.sql CREATE_ADMIN=true go run cmd/runsql/main.go

# Drop all database data by running the Go executable on the host system
dropdb:
    SQL_FILE=./db/DropTables.sql CREATE_ADMIN=false go run cmd/runsql/main.go

format:
    go tool goimports -l -w cmd internal pkg
    go tool gofumpt -l -w cmd internal pkg
    go tool golines -w -m 120 cmd internal pkg

lint:
    golangci-lint run ./cmd/... ./internal/... ./pkg/...
