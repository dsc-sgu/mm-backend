package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

func TestCreateBlock(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	draftSnapshotID := LockCourse(
		t,
		&backendPort,
		&testUser,
		courseID,
	)
	require.NotZero(t, draftSnapshotID)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		draftSnapshotID,
	)
	require.NotZero(t, blockID)
}

func TestGetBlockByID(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	draftSnapshotID := LockCourse(
		t,
		&backendPort,
		&testUser,
		courseID,
	)
	require.NotZero(t, draftSnapshotID)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		draftSnapshotID,
	)

	getBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/blocks/%s",
		backendPort.Port(),
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

	var returnedBlock blocks.Block
	require.NoError(
		t,
		json.NewDecoder(getBlockResp.Body).Decode(&returnedBlock),
	)

	require.Equal(t, draftSnapshotID, returnedBlock.SnapshotID)
}

func GetBlockByID(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	blockID uuid.UUID,
) blocks.Block {
	t.Helper()

	getBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/blocks/%s",
		port.Port(),
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

func TestDeleteBlock(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	draftSnapshotID := LockCourse(
		t,
		&backendPort,
		&testUser,
		courseID,
	)
	require.NotZero(t, draftSnapshotID)

	// Create two blocks
	block1ID := CreateTestBlock(t, &backendPort, &testUser, draftSnapshotID)
	block2ID := CreateTestBlock(t, &backendPort, &testUser, draftSnapshotID)
	require.NotZero(t, block1ID)
	require.NotZero(t, block2ID)

	// Verify we have 2 blocks
	snapshotBlocks := GetSnapshotBlocks(t, &backendPort, &testUser, draftSnapshotID)
	require.Len(t, snapshotBlocks, 2)

	// Delete one block
	deleteBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/snapshots/%s/blocks/%s",
		backendPort.Port(),
		draftSnapshotID,
		block1ID,
	)

	deleteBlockReq, err := http.NewRequest(
		http.MethodDelete,
		deleteBlockURL,
		nil,
	)
	require.NoError(t, err)

	deleteBlockResp, err := testUser.Client.Do(deleteBlockReq)
	require.NoError(t, err)

	defer func() {
		err := deleteBlockResp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNoContent, deleteBlockResp.StatusCode)

	// Verify that the number of active blocks is now 1
	snapshotBlocks = GetSnapshotBlocks(t, &backendPort, &testUser, draftSnapshotID)
	require.Len(t, snapshotBlocks, 1)
	require.Equal(t, block2ID, snapshotBlocks[0].ID)
}
