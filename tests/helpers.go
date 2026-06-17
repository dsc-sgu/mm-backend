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
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

// Helper functions for tests
func initBackend(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
) (testcontainers.Container, *nat.Port, error) {
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
			"HOST":          "0.0.0.0",
			"HTTP_PORT":     basePort.Port(),
			"POSTGRES_PORT": "5432",
			"POSTGRES_HOST": "mm-postgres",
			"REDIS_HOST":    "mm-redis",
			"REDIS_PORT":    "6379",
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
		return nil, nil, err
	}

	port, err := container.MappedPort(ctx, nat.Port(basePort))
	if err != nil {
		return nil, nil, err
	}

	return container, &port, nil
}

func initPostgres(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
) (testcontainers.Container, *nat.Port, error) {
	dbUser := "postgres"
	dbPassword := "postgres"
	dbName := "postgres"
	pgPort := nat.Port("5432/tcp")
	SQLPath, err := filepath.Abs("../db/CreateTables.sql")
	if err != nil {
		return nil, nil, err
	}
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
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort(pgPort),
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
		return nil, nil, err
	}

	port, err := container.MappedPort(ctx, pgPort)
	if err != nil {
		return nil, nil, err
	}

	return container, &port, nil
}

func initRedis(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
) (testcontainers.Container, *nat.Port, error) {
	redisPort := nat.Port("6379/tcp")

	req := testcontainers.ContainerRequest{
		Name:         "mm-redis",
		Image:        "valkey/valkey:8",
		ExposedPorts: []string{string(redisPort)},
		WaitingFor:   wait.ForListeningPort(redisPort),
		Networks:     []string{net.Name},
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	port, err := container.MappedPort(ctx, redisPort)
	if err != nil {
		return nil, nil, err
	}

	return container, &port, nil
}

func clearPostgres(t *testing.T) {
	t.Helper()

	_, err := testPostgres.Exec(`
		TRUNCATE TABLE
			users,
			disciplines,
			courses,
			course_members,
			students,
			teachers,
			invites,
			blocks
		RESTART IDENTITY CASCADE;
	`)
	require.NoError(t, err)
}

func clearRedis(t *testing.T) {
	t.Helper()

	err := testRedis.FlushDB(context.Background()).Err()
	require.NoError(t, err)
}

func clearDatabases(t *testing.T) {
	t.Helper()
	clearPostgres(t)
	clearRedis(t)
}

func CreateTestUser(
	t *testing.T,
	port *nat.Port,
	firstName, lastName, username, email, password string,
) uuid.UUID {
	t.Helper()

	userURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/auth/register",
		port.Port(),
	)

	userBody, _ := json.Marshal(users.UserRegisterRequest{
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

	return createdUser.ID
}

func CreateTestDiscipline(
	t *testing.T,
	port *nat.Port,
	userID uuid.UUID,
	name string,
) uuid.UUID {
	t.Helper()

	disciplineURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/disciplines?fake_user_id=%s",
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

	return createdDiscipline.ID
}

func CreateTestCourse(
	t *testing.T,
	port *nat.Port,
	userID uuid.UUID,
	disciplineID uuid.UUID,
	name string,
	displayName string,
) uuid.UUID {
	t.Helper()

	courseURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses?fake_user_id=%s",
		port.Port(),
		userID,
	)

	courseBody, _ := json.Marshal(courses.CreateCourse{
		DisciplineID: disciplineID,
		Name:         name,
		DisplayName:  displayName,
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

	return createdCourse.ID
}

func CreateTestBlock(
	t *testing.T,
	port *nat.Port,
	userID uuid.UUID,
	courseID uuid.UUID,
) uuid.UUID {
	t.Helper()

	blockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/blocks/%s/blocks?fake_user_id=%s",
		port.Port(),
		courseID,
		userID,
	)

	blockBody, err := json.Marshal(blocks.CreateBlock{
		CourseID:  courseID,
		BlockType: "test",
		Data:      []byte("true"),
	})
	require.NoError(t, err)

	blockReq, err := http.NewRequest(
		http.MethodPost,
		blockURL,
		bytes.NewBuffer(blockBody),
	)
	require.NoError(t, err)

	blockReq.Header.Set("Content-Type", "application/json")

	blockResp, err := http.DefaultClient.Do(blockReq)
	require.NoError(t, err)
	defer func() {
		err := blockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusCreated, blockResp.StatusCode)

	var createdBlock blocks.Block
	require.NoError(t, json.NewDecoder(blockResp.Body).Decode(&createdBlock))

	return createdBlock.ID
}

func GetRoleInCourse(
	t *testing.T,
	port *nat.Port,
	userID, courseID uuid.UUID,
) *courses.CourseMemberRole {
	t.Helper()

	roleURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/roles/%s?fake_user_id=%s",
		port.Port(),
		courseID,
		userID,
	)
	roleReq, err := http.NewRequest(http.MethodGet, roleURL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(roleReq)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	if resp.StatusCode == http.StatusInternalServerError {
		return nil
	}

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var roleResp courses.UserRoleResponse
	err = json.NewDecoder(resp.Body).Decode(&roleResp)
	require.NoError(t, err)

	return &roleResp.Role
}
