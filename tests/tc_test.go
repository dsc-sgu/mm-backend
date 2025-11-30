// Package tests should contain integration tests for the backend.
package tests

import (
	"context"
	// "encoding/json"
	// "io"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func initBackend(
	ctx context.Context,
	t *testing.T,
	net *testcontainers.DockerNetwork,
) (*nat.Port, error) {
	basePort := "5432/tcp"

	req := testcontainers.ContainerRequest{
		Name: "mm-backend",
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "..",
			Dockerfile: "Dockerfile",
		},
		Entrypoint:   []string{"/app/server"},
		ExposedPorts: []string{basePort},
		Env: map[string]string{
			"POSTGRES_PORT": "5432",
			"POSTGRES_HOST": "mm-postgres",
		},
		WaitingFor: wait.ForLog("Server running"),
		Networks:   []string{net.Name},
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

	port, err := container.MappedPort(ctx, nat.Port(basePort))

	if err != nil {
		return nil, err
	}

	return &port, nil
}

func initPostgres(
	ctx context.Context,
	t *testing.T,
	net *testcontainers.DockerNetwork,
) (*nat.Port, error) {
	dbUser := "postgres"
	dbPassword := "postgres"
	dbName := "postgres"
	dbPort := "5432/tcp"

	req := testcontainers.ContainerRequest{
		Name:         "mm-postgres",
		Image:        "postgres:latest",
		ExposedPorts: []string{dbPort},
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForListeningPort(nat.Port(dbPort)),
		Networks:   []string{net.Name},
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

	port, err := container.MappedPort(ctx, nat.Port(dbPort))

	if err != nil {
		return nil, err
	}

	return &port, nil
}

//func sendRequest(
//	t *testing.T,
//net *testcontainers.DockerNetwork,
//	//
//)

// func readBodyToMap(rc io.ReadCloser) (map[string]any, error) {
// 	body, err := io.ReadAll(rc)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer func() {
// 		_ = rc.Close()
// 	}()

// 	var result map[string]any
// 	if err := json.Unmarshal(body, &result); err != nil {
// 		return nil, err
// 	}
// 	return result, nil
// }

func TestInitBackend(t *testing.T) {
	ctx := context.Background()
	network, err := network.New(ctx)
	assert.Nil(t, err)
	_, _ = initPostgres(ctx, t, network)
	port, err := initBackend(ctx, t, network)
	if err != nil {
		t.Fatalf("initBackend error: %v", err)
	}
	t.Logf("Backend started on port: %v", port.Port())
}

func TestInitPostgres(t *testing.T) {
	ctx := context.Background()
	network, err := network.New(ctx)
	assert.Nil(t, err)
	port, err := initPostgres(ctx, t, network)
	if err != nil {
		t.Fatalf("initPostgres error: %v", err)
	}
	t.Logf("Postgres started on port: %v", port.Port())
}
