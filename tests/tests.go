// Package tests should contain integration tests for the backend.
package tests

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func initBackend(
	ctx context.Context,
	t *testing.T,
	net *testcontainers.DockerNetwork,
) (*nat.Port, error) {
	basePort := "8013/tcp"

	req, err := testcontainers.ContainerRequest{
		Image:        "mm-backend",
		ExposedPorts: []string{basePort},
		WaitingFor:   wait.ForListeningPort(basePort),
	}

	if err != nil {
		return nil, err
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)

	if err != nil {
		return nil, err
	}

	testcontainers.CleanupContainer(t, container)

	return container.MappedPort(ctx, basePort)
}

func initPostgres(
	ctx context.Context,
	t *testing.T,
) (nat.Port, error) {
	dbUser := "sguhack"
	dbPassword := "postgres"
	dbName := "sguhack"
	dbPort := "5432/tcp"

	req, err := testcontainers.ContainerRequest{
		Image:        "postgres:latest",
		ExposedPorts: []string{dbPort},
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForListeningPort(dbPort),
	}

	if err != nil {
		return nil, err
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})

	if err != nil {
		return nil, err
	}

	testcontainers.CleanupContainer(t, container)

	return container.MappedPort(ctx, dbPort)
}

func sendRequest(
	t *testing.T,
	net *testcontainers.DockerNetwork,
	//
)

func readBodyToMap(rc io.ReadCloser) (map[string]any, error) {
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}
