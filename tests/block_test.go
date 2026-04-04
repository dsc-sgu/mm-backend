package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	).DraftSnapshotID
	require.NotZero(t, draftSnapshotID)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		courseID,
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
	).DraftSnapshotID
	require.NotZero(t, draftSnapshotID)

	blockID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)

	returnedBlock := GetBlockByID(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
		blockID,
	)

	require.Equal(t, draftSnapshotID, returnedBlock.SnapshotID)
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
	).DraftSnapshotID
	require.NotZero(t, draftSnapshotID)

	// Create two blocks
	block1ID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)
	block2ID := CreateTestBlock(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)
	require.NotZero(t, block1ID)
	require.NotZero(t, block2ID)

	// Verify we have 2 blocks
	snapshotBlocks := GetSnapshotBlocks(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)
	require.Len(t, snapshotBlocks, 2)

	// Delete one block
	deleteBlockURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/snapshots/%s/blocks/%s",
		backendPort.Port(),
		courseID,
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
	snapshotBlocks = GetSnapshotBlocks(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)
	require.Len(t, snapshotBlocks, 1)
	require.Equal(t, block2ID, snapshotBlocks[0].ID)
}

func TestBlockLexoRankOrdering(t *testing.T) {
	clearDatabases(t)

	// --- Setup ---
	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Lexo",
		"User",
		"lexo",
		"lexo@test.com",
		"password",
	)
	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Lexo Discipline",
	)
	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Lexo Course",
	)
	draftSnapshotID := LockCourse(
		t,
		&backendPort,
		&testUser,
		courseID,
	).DraftSnapshotID

	var expectedOrder []uuid.UUID
	var lastBlockID uuid.UUID

	// --- Stage 1: Insert first block ---
	lastBlockID = CreateTestBlockAfter(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
		nil,
	)
	expectedOrder = append(expectedOrder, lastBlockID)

	// --- Stage 2: Insert 5 blocks at the top ---
	var topBlocks []uuid.UUID
	for range 5 {
		newID := CreateTestBlockAfter(
			t,
			&backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
			nil,
		)
		topBlocks = append(
			[]uuid.UUID{newID},
			topBlocks...,
		) // Prepending to get the correct final order
	}
	expectedOrder = append(topBlocks, expectedOrder...)

	// --- Stage 3: Insert 5 blocks at the bottom ---
	for range 5 {
		lastBlockID = CreateTestBlockAfter(
			t,
			&backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
			&lastBlockID,
		)
		expectedOrder = append(expectedOrder, lastBlockID)
	}

	// --- Stage 4: Insert 5 blocks in the middle (after the very first block) ---
	middleAnchorID := expectedOrder[5] // The first block created is now at index 5
	var middleBlocks []uuid.UUID
	for range 5 {
		newID := CreateTestBlockAfter(
			t,
			&backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
			&middleAnchorID,
		)
		middleBlocks = append(middleBlocks, newID)
		middleAnchorID = newID
	}
	// Insert middle blocks into expected order
	expectedOrder = append(
		expectedOrder[:6],
		append(middleBlocks, expectedOrder[6:]...)...,
	)

	// --- Verification ---
	finalBlocks := GetSnapshotBlocks(
		t,
		&backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
	)
	require.Len(t, finalBlocks, 16)

	var finalOrderIDs []uuid.UUID
	for _, b := range finalBlocks {
		finalOrderIDs = append(finalOrderIDs, b.ID)
	}
	require.Equal(
		t,
		expectedOrder,
		finalOrderIDs,
		"Blocks are not in the correct order after insertion",
	)

	// --- Stage 5: Test moving blocks ---
	t.Run("Move Blocks", func(t *testing.T) {
		// Initial state: B0-B4 (top), M0-M4 (middle), I (initial), L0-L4 (last)
		// For simplicity, let's just grab the IDs from the verified finalOrderIDs
		blockToMove := finalOrderIDs[15] // last4
		afterBlock := finalOrderIDs[0]   // top4

		// Move last block to be after the first block
		MoveBlockAfter(
			t,
			&backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
			blockToMove,
			&afterBlock,
		)

		// Recalculate expected order
		expectedAfterMove := []uuid.UUID{finalOrderIDs[0], finalOrderIDs[15]}
		expectedAfterMove = append(expectedAfterMove, finalOrderIDs[1:15]...)

		// Verify new order
		blocksAfterMove := GetSnapshotBlocks(
			t,
			&backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
		)
		var idsAfterMove []uuid.UUID
		for _, b := range blocksAfterMove {
			idsAfterMove = append(idsAfterMove, b.ID)
		}
		require.Equal(
			t,
			expectedAfterMove,
			idsAfterMove,
			"Blocks are not in correct order after moving",
		)
	})
}
