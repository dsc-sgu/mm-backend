package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

func TestCreateBlock(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		userID,
		courseID,
	)
	require.NotZero(t, blockID)
}

func TestGetBlockByID(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		userID,
		courseID,
	)

	getBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/blocks/%s?fake_user_id=%s",
		backendPort.Port(),
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

	var returnedBlock blocks.Block
	require.NoError(
		t,
		json.NewDecoder(getBlockResp.Body).Decode(&returnedBlock),
	)

	require.Equal(t, courseID, returnedBlock.CourseID)
}

func TestUnlinkBlock(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		userID,
		courseID,
	)

	unlinkBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/blocks/%s/%s?fake_user_id=%s",
		backendPort.Port(),
		blockID,
		courseID,
		userID,
	)

	unlinkBlockReq, err := http.NewRequest(
		http.MethodDelete,
		unlinkBlockURL,
		nil,
	)
	require.NoError(t, err)

	unlinkBlockResp, err := http.DefaultClient.Do(unlinkBlockReq)
	require.NoError(t, err)

	defer func() {
		err := unlinkBlockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	var returnedBlock blocks.Block
	require.NoError(
		t,
		json.NewDecoder(unlinkBlockResp.Body).Decode(&returnedBlock),
	)

	require.Equal(t, uuid.Nil, returnedBlock.CourseID)
}
