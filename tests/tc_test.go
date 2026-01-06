// Package tests should contain integration tests for the backend.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
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
	SQLPath, err := filepath.Abs("../db/CreateTables.sql")
	require.NoError(t, err)
	fmt.Println(SQLPath)

	req := testcontainers.ContainerRequest{
		Name:         "mm-postgres",
		Image:        "postgres:18.1",
		ExposedPorts: []string{string(pgPort)},
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
			"POSTGRES_DB":       dbName,
		},
		Tmpfs: map[string]string{
			"/var/lib/postgresql": "rw",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      SQLPath,
				ContainerFilePath: "/docker-entrypoint-initdb.d/CreateTables.sql",
				FileMode:          0o644,
			},
		},
		WaitingFor: wait.ForLog(
			"database system is ready to accept connections",
		),
		Networks: []string{net.Name},
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

func CreateTestUser(
	t *testing.T,
	port *nat.Port,
	firstName, lastName, username, email, password string,
) uuid.UUID {
	userURL := fmt.Sprintf("http://localhost:%s/api/v1/auth/register",
		port.Port(),
	)

	userBody, _ := json.Marshal(users.RegisterModel{
		FirstName: firstName,
		LastName:  lastName,
		Username:  username,
		Email:     email,
		Password:  password,
	})

	userReq, err := http.NewRequest(
		http.MethodPost,
		userURL,
		bytes.NewBuffer(userBody),
	)
	require.NoError(t, err)
	userReq.Header.Set("Content-Type", "application/json")

	userResp, err := http.DefaultClient.Do(userReq)
	require.NoError(t, err)
	defer func() {
		err := userResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusCreated, userResp.StatusCode)

	var createdUser users.RegisterResponse
	require.NoError(t, json.NewDecoder(userResp.Body).Decode(&createdUser))

	return createdUser.Id
}

func CreateTestDiscipline(
	t *testing.T,
	port *nat.Port,
	userID uuid.UUID,
	name string,
) uuid.UUID {
	disciplineURL := fmt.Sprintf(
		"http://localhost:%s/api/v1/disciplines?fake_user_id=%s",
		port.Port(),
		userID,
	)

	disciplineBody, _ := json.Marshal(disciplines.CreateDiscipline{
		Name: name,
	})

	disciplineReq, err := http.NewRequest(
		http.MethodPost,
		disciplineURL,
		bytes.NewBuffer(disciplineBody),
	)
	require.NoError(t, err)
	disciplineReq.Header.Set("Content-Type", "application/json")

	disciplineResp, err := http.DefaultClient.Do(disciplineReq)
	require.NoError(t, err)
	defer func() {
		err := disciplineResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusCreated, disciplineResp.StatusCode)

	var createdDiscipline disciplines.Discipline
	require.NoError(
		t,
		json.NewDecoder(disciplineResp.Body).Decode(&createdDiscipline),
	)

	return createdDiscipline.Id
}

func CreateTestCourse(
	t *testing.T,
	port *nat.Port,
	userID uuid.UUID,
	disciplineID uuid.UUID,
	name string,
) uuid.UUID {
	courseURL := fmt.Sprintf(
		"http://localhost:%s/api/v1/courses?fake_user_id=%s",
		port.Port(),
		userID,
	)

	courseBody, _ := json.Marshal(courses.CreateCourse{
		DisciplineId: disciplineID,
		Name:         name,
	})

	courseReq, err := http.NewRequest(
		http.MethodPost,
		courseURL,
		bytes.NewBuffer(courseBody),
	)
	require.NoError(t, err)
	courseReq.Header.Set("Content-Type", "application/json")

	courseResp, err := http.DefaultClient.Do(courseReq)
	require.NoError(t, err)
	defer func() {
		err := courseResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusCreated, courseResp.StatusCode)

	var createdCourse courses.Course
	require.NoError(t, json.NewDecoder(courseResp.Body).Decode(&createdCourse))

	return createdCourse.Id
}

func TestCreateCourse(t *testing.T) {
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)

	_, err = initPostgres(ctx, t, net)
	require.NoError(t, err)

	port, err := initBackend(ctx, t, net)
	require.NoError(t, err)

	userID := CreateTestUser(
		t,
		port,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID := CreateTestDiscipline(
		t,
		port,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		port,
		userID,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)
}
