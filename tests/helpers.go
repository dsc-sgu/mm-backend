package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/auth/users"
	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	"github.com/dsc-sgu/mm-backend/internal/disciplines"
)

type TestUser struct {
	ID        uuid.UUID
	Client    *http.Client
	SessionID uuid.UUID
}

// Helper functions for tests
func initBackend(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
) (testcontainers.Container, *nat.Port, error) {
	return initBackendWithEnv(ctx, net, nil)
}

func initBackendWithEnv(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
	additionalEnv map[string]string,
) (testcontainers.Container, *nat.Port, error) {
	basePort := nat.Port("80/tcp")

	env := map[string]string{
		"HOST":          "0.0.0.0",
		"HTTP_PORT":     basePort.Port(),
		"POSTGRES_PORT": "5432",
		"POSTGRES_HOST": "mm-postgres",
		"REDIS_HOST":    "mm-redis",
		"REDIS_PORT":    "6379",
		"ENABLE_AUTH":   "true",
	}

	maps.Copy(env, additionalEnv)

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "..",
			Dockerfile: "Dockerfile",
		},
		Entrypoint:   []string{"/app/server"},
		ExposedPorts: []string{string(basePort)},
		Env:          env,
		WaitingFor:   wait.ForLog("Server running"),
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
		Networks:       []string{net.Name},
		NetworkAliases: map[string][]string{net.Name: {"mm-postgres"}},
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
		Image:          "valkey/valkey:8",
		ExposedPorts:   []string{string(redisPort)},
		WaitingFor:     wait.ForListeningPort(redisPort),
		Networks:       []string{net.Name},
		NetworkAliases: map[string][]string{net.Name: {"mm-redis"}},
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
			course_locks,
			course_snapshots,
			blocks,
			courses,
			course_members,
			users,
			disciplines,
			students,
			teachers,
			invites
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

	userURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/auth/register",
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

func LoginUser(
	t *testing.T,
	port *nat.Port,
	email, password string,
) TestUser {
	t.Helper()

	loginURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/auth/login",
		port.Port(),
	)

	loginBody, _ := json.Marshal(users.LoginUser{
		Email:    email,
		Password: password,
	})

	loginReq, err := http.NewRequest(
		http.MethodPost,
		loginURL,
		bytes.NewBuffer(loginBody),
	)
	require.NoError(t, err)
	loginReq.Header.Set("Content-Type", "application/json")

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	loginResp, err := client.Do(loginReq)
	require.NoError(t, err)
	defer func() {
		err := loginResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	var loginResponse users.LoginResponse
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginResponse))

	var sessionID uuid.UUID
	for _, cookie := range loginResp.Cookies() {
		if cookie.Name == session.CookieName {
			sessionID, err = uuid.Parse(cookie.Value)
			require.NoError(t, err)
			break
		}
	}

	require.NotEqual(t, uuid.Nil, sessionID)

	return TestUser{
		ID:        loginResponse.UserID,
		Client:    client,
		SessionID: sessionID,
	}
}

func CreateAndLoginUser(
	t *testing.T,
	port *nat.Port,
	firstName, lastName, username, email, password string,
) TestUser {
	t.Helper()

	userID := CreateTestUser(
		t,
		port,
		firstName,
		lastName,
		username,
		email,
		password,
	)
	testUser := LoginUser(t, port, email, password)
	require.Equal(t, userID, testUser.ID)

	return testUser
}

func CreateTestDiscipline(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	name string,
) uuid.UUID {
	t.Helper()

	disciplineURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/disciplines",
		port.Port(),
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

	disciplineResp, err := testUser.Client.Do(disciplineReq)
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
	testUser *TestUser,
	disciplineID uuid.UUID,
	name string,
	displayName string,
) uuid.UUID {
	t.Helper()

	courseURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses",
		port.Port(),
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

	courseResp, err := testUser.Client.Do(courseReq)
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
	testUser *TestUser,
	courseID, snapshotID uuid.UUID,
) uuid.UUID {
	return CreateTestBlockAfter(t, port, testUser, courseID, snapshotID, nil)
}

func CreateTestBlockAfter(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID, snapshotID uuid.UUID,
	afterBlockID *uuid.UUID,
) uuid.UUID {
	t.Helper()

	blockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/snapshots/%s/blocks",
		port.Port(),
		courseID,
		snapshotID,
	)

	blockBody, err := json.Marshal(blocks.CreateBlock{
		AfterBlockID: afterBlockID,
		BlockType:    "text",
		Data:         []byte("true"),
	})
	require.NoError(t, err)

	blockReq, err := http.NewRequest(
		http.MethodPost,
		blockURL,
		bytes.NewBuffer(blockBody),
	)
	require.NoError(t, err)

	blockReq.Header.Set("Content-Type", "application/json")

	blockResp, err := testUser.Client.Do(blockReq)
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

func MoveBlockAfter(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID, snapshotID, blockToMoveID uuid.UUID,
	afterBlockID *uuid.UUID,
) {
	t.Helper()

	moveURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/snapshots/%s/blocks/%s/move",
		port.Port(),
		courseID,
		snapshotID,
		blockToMoveID,
	)

	body, _ := json.Marshal(struct {
		AfterBlockID *uuid.UUID `json:"afterBlockId"`
	}{
		AfterBlockID: afterBlockID,
	})

	req, err := http.NewRequest(
		http.MethodPatch,
		moveURL,
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func GetBlockByID(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID, snapshotID, blockID uuid.UUID,
) blocks.Block {
	t.Helper()

	getBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/snapshots/%s/blocks/%s",
		port.Port(),
		courseID,
		snapshotID,
		blockID,
	)

	getBlockReq, err := http.NewRequest(
		http.MethodGet,
		getBlockURL,
		nil,
	)
	require.NoError(t, err)

	getBlockResp, err := testUser.Client.Do(getBlockReq)
	require.NoError(t, err)

	defer func() {
		err := getBlockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, getBlockResp.StatusCode)

	var returnedBlock blocks.Block
	require.NoError(
		t,
		json.NewDecoder(getBlockResp.Body).Decode(&returnedBlock),
	)

	return returnedBlock
}

func GetRoleInCourse(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) *membership.Role {
	t.Helper()

	roleURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/role",
		port.Port(),
		courseID,
	)
	roleReq, err := http.NewRequest(http.MethodGet, roleURL, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(roleReq)
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

type LockCourseResult struct {
	InitType        string    `json:"initType"`
	DraftSnapshotID uuid.UUID `json:"draftSnapshotID"`
}

func LockCourse(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) LockCourseResult {
	t.Helper()

	lockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/lock",
		port.Port(),
		courseID,
	)

	lockReq, err := http.NewRequest(http.MethodPost, lockURL, nil)
	require.NoError(t, err)
	lockReq.Header.Set("Content-Type", "application/json")

	lockResp, err := testUser.Client.Do(lockReq)
	require.NoError(t, err)
	defer func() {
		err := lockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, lockResp.StatusCode)

	var lockResult LockCourseResult
	require.NoError(t, json.NewDecoder(lockResp.Body).Decode(&lockResult))

	require.NotEqual(t, uuid.Nil, lockResult.DraftSnapshotID)

	return lockResult
}
