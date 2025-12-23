# MergeMinds backend

## Development

### Requirements

- Install `docker` and `docker compose`
- Install `just` to use `justfile` recipes
- (optional, but desired) Install `air` (for hot reload)

### Starting application

1. Run `just dev`, it will start all required databases via `docker compose`
2. Run application with `go run ./cmd/server/main.go` or `air`

### Tests

**Unit tests** are located inside packages, which they're related to, so you
can run all unit tests with `just test`

**Functional tests** cover primary API usage scenarios and are located inside `tests` directory.
You can run them with `just functional-test`
