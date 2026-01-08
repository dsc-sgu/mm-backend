package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

func TestCreateBlock(t *testing.T) {
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

	blockID := CreateTestBlock(
		t,
		port,
		userID,
		courseID,
	)
	require.NotZero(t, blockID)
}

func TestGetBlockById(t *testing.T) {
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

	blockID := CreateTestBlock(
		t,
		port,
		userID,
		courseID,
	)

	getBlockURL := fmt.Sprintf(
		"http://localhost:%s/api/v1/blocks/%s?fake_user_id=%s",
		port.Port(),
		blockID,
		userID,
	)

	getBlockReq, err := http.NewRequest(
		http.MethodGet,
		getBlockURL,
		nil,
	)
	require.NoError(t, err)

	getBlockResp, err := http.DefaultClient.Do(getBlockReq)
	require.NoError(t, err)

	defer func() {
		err := getBlockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	var createdBlock blocks.Block
	require.NoError(t, json.NewDecoder(getBlockResp.Body).Decode(&createdBlock))

	require.Equal(t, courseID, createdBlock.CourseId)
}
