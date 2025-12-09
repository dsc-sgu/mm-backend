// Package tests should contain integration tests for the backend.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

func initBackend(
	ctx context.Context,
	t *testing.T,
	net *testcontainers.DockerNetwork,
) (*nat.Port, error) {
	basePort := nat.Port("80/tcp")

	req := testcontainers.ContainerRequest{
		Name: "mm-backend",
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "..",
			Dockerfile: "Dockerfile",
		},
		Entrypoint:   []string{"/app/server"},
		ExposedPorts: []string{string(basePort)},
		Env: map[string]string{
			"HTTP_PORT":     basePort.Port(),
			"POSTGRES_PORT": "5432",
			"POSTGRES_HOST": "mm-postgres",
			"ENABLE_AUTH":   "false",
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
	pgPort := nat.Port("5432/tcp")

	req := testcontainers.ContainerRequest{
		Name:         "mm-postgres",
		Image:        "postgres:latest",
		ExposedPorts: []string{string(pgPort)},
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForListeningPort(pgPort),
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

	port, err := container.MappedPort(ctx, pgPort)
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

func TestCreateCourse(t *testing.T) {
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)

	_, err = initPostgres(ctx, t, net)
	require.NoError(t, err)

	port, err := initBackend(ctx, t, net)
	require.NoError(t, err)

	url := fmt.Sprintf("http://localhost:%s/api/v1/courses?fake_user_id=\"%s\"",
		port.Port(),
		uuid.New().String(),
	)

	body, _ := json.Marshal(courses.CreateCourse{
    DisciplineId: uuid.New(),
    Name:         "Test Course",
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, 201, resp.StatusCode)

	var created courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	require.Equal(t, "Test course ID", created.Id)
	require.Equal(t, "Test course name:", created.Name)
}
