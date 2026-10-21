package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

// setupCourseForBlocks creates a teacher with a course and returns both.
func setupCourseForBlocks(t *testing.T) (TestUser, uuid.UUID) {
	t.Helper()

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
	return testUser, courseID
}

func TestCreateBlock(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)

	blockID := CreateTestBlock(t, &backendPort, &testUser, courseID)
	require.NotZero(t, blockID)
}

// TestGetBlockByID checks that a new block is a draft of its author in
// version 1, and that it stays a draft until published.
func TestGetBlockByID(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)

	blockID := CreateTestBlock(t, &backendPort, &testUser, courseID)

	returnedBlock := GetBlockByID(t, &backendPort, &testUser, courseID, blockID)
	require.Equal(t, blockID, returnedBlock.ID)
	require.Equal(t, courseID, returnedBlock.CourseID)
	require.Equal(t, 1, returnedBlock.Version)
	require.True(t, returnedBlock.Draft)
	require.Equal(t, testUser.ID, returnedBlock.CreatedBy)

	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	returnedBlock = GetBlockByID(t, &backendPort, &testUser, courseID, blockID)
	require.Equal(t, 1, returnedBlock.Version)
	require.False(t, returnedBlock.Draft)
}

// TestPatchPublishedBlockCreatesDraftVersion checks that editing a published
// block leaves it as it is for readers and gives the author a draft version
// of their own.
func TestPatchPublishedBlockCreatesDraftVersion(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)

	blockID := CreateTestBlock(t, &backendPort, &testUser, courseID)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	require.Equal(t, http.StatusOK, PatchTestBlockData(
		t, &backendPort, &testUser, courseID, blockID, `{"text":"edited","version":1}`,
	))

	draft := GetBlockByID(t, &backendPort, &testUser, courseID, blockID)
	require.Equal(t, 2, draft.Version)
	require.True(t, draft.Draft)
	require.JSONEq(t, `{"text":"edited","version":1}`, string(draft.Data))

	published := GetCourseContent(t, &backendPort, &testUser, courseID).Blocks
	require.Len(t, published, 1)
	require.Equal(t, 1, published[0].Version)
	require.JSONEq(t, `{"version":1}`, string(published[0].Data))
}

func TestDeleteBlock(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)

	// Create two blocks
	block1ID := CreateTestBlock(t, &backendPort, &testUser, courseID)
	block2ID := CreateTestBlock(t, &backendPort, &testUser, courseID)
	require.NotZero(t, block1ID)
	require.NotZero(t, block2ID)

	// Verify we have 2 blocks
	draftBlocks := GetDraftBlocks(t, &backendPort, &testUser, courseID)
	require.Len(t, draftBlocks, 2)

	// Delete one block
	require.Equal(t, http.StatusNoContent, DeleteTestBlock(
		t, &backendPort, &testUser, courseID, block1ID,
	))

	// Verify that the number of active blocks is now 1
	draftBlocks = GetDraftBlocks(t, &backendPort, &testUser, courseID)
	require.Len(t, draftBlocks, 1)
	require.Equal(t, block2ID, draftBlocks[0].ID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks/%s",
		backendPort.Port(),
		courseID,
		block1ID,
	)
	resp, err := testUser.Client.Get(url)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestBlockLexoRankOrdering(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)

	var expectedOrder []uuid.UUID
	var lastBlockID uuid.UUID

	// --- Stage 1: Insert first block ---
	lastBlockID = CreateTestBlockAfter(t, &backendPort, &testUser, courseID, nil)
	expectedOrder = append(expectedOrder, lastBlockID)

	// --- Stage 2: Insert 5 blocks at the top ---
	var topBlocks []uuid.UUID
	for range 5 {
		newID := CreateTestBlockAfter(t, &backendPort, &testUser, courseID, nil)
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
	finalBlocks := GetDraftBlocks(t, &backendPort, &testUser, courseID)
	require.Len(t, finalBlocks, 16)

	finalOrderIDs := blockIDs(finalBlocks)
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
			blockToMove,
			&afterBlock,
		)

		// Recalculate expected order
		expectedAfterMove := []uuid.UUID{finalOrderIDs[0], finalOrderIDs[15]}
		expectedAfterMove = append(expectedAfterMove, finalOrderIDs[1:15]...)

		// Verify new order
		blocksAfterMove := GetDraftBlocks(t, &backendPort, &testUser, courseID)
		require.Equal(
			t,
			expectedAfterMove,
			blockIDs(blocksAfterMove),
			"Blocks are not in correct order after moving",
		)
	})
}

// TestPatchBlockTypeIsImmutable checks that a block cannot change its type.
func TestPatchBlockTypeIsImmutable(t *testing.T) {
	clearDatabases(t)

	testUser, courseID := setupCourseForBlocks(t)
	blockID := CreateTestBlock(t, &backendPort, &testUser, courseID)

	heading := "heading"
	body, err := json.Marshal(blocks.PatchBlockCommand{Version: 1, BlockType: &heading})
	require.NoError(t, err)
	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks/%s",
		backendPort.Port(),
		courseID,
		blockID,
	)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	block := GetBlockByID(t, &backendPort, &testUser, courseID, blockID)
	require.Equal(t, "paragraph", block.BlockType)
}

// TestCreateBlockValidatesTypeAndData checks the two things the backend knows
// about a block: the shape of its type name and that its data is a versioned
// JSON object.
func TestCreateBlockValidatesTypeAndData(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)
	cases := []struct {
		name      string
		blockType string
		data      string
		status    int
	}{
		{"domain type", "list-item", `{"version":1}`, http.StatusCreated},
		{"uppercase type", "Paragraph", `{"version":1}`, http.StatusBadRequest},
		{"too long type", strings.Repeat("a", 65), `{"version":1}`, http.StatusBadRequest},
		{"data without version", "paragraph", `{"text":"a"}`, http.StatusBadRequest},
		{"no data", "paragraph", ``, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, err := json.Marshal(blocks.CreateBlockCommand{
				BlockType: c.blockType,
				Data:      json.RawMessage(c.data),
			})
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			resp, err := testUser.Client.Do(req)
			require.NoError(t, err)
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			require.Equal(t, c.status, resp.StatusCode)
		})
	}
}
