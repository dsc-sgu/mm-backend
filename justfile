# Run the application in Docker Compose without initializing the database. This requires manual initialization via 'just initdb-compose'
run-compose:
    docker compose up backend db dragonfly

# Run the application in Docker Compose with Air (for development)
dev:
    docker compose up backend-dev db dragonfly

# Initialize the database when running in Docker Compose: creates tables and admin user
initdb-compose:
    docker compose run --rm initdb

# Drop all database data when running in Docker Compose
dropdb-compose:
    docker compose run --rm dropdb

# Run the application in Docker Compose with a clean database state by first dropping and reinitializing
full-run-compose: dropdb-compose initdb-compose run-compose

# Run the database in a separate Docker container
rundb-docker DB_NAME='mmdb' DB_USER='mmuser':
    docker run --rm --name psql -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB={{DB_NAME}} -e POSTGRES_USER={{DB_USER}} --network=host postgres:17.1-alpine

# Run Dragonfly in a separate Docker container
runfly-docker:
    docker run --rm --name dragonfly --ulimit memlock=-1 --network=host docker.dragonflydb.io/dragonflydb/dragonfly

# Initialize the database by running the Go executable on the host system
initdb-host:
    SQL_FILE=./db/CreateTables.sql CREATE_ADMIN=true go run cmd/runsql/main.go

# Drop all database data by running the Go executable on the host system
dropdb-host:
    SQL_FILE=./db/DropTables.sql CREATE_ADMIN=false go run cmd/runsql/main.go

# Run the application on the host system
run-host:
    go run cmd/webserver/main.go


# Install Git hooks
precommit-install:
    pre-commit install

format:
    go tool goimports -l -w cmd internal pkg
    go tool gofumpt -l -w cmd internal pkg
    go tool golines -w -m 120 cmd internal pkg

lint:
    @golangci-lint run ./cmd/... ./internal/... ./pkg/...
