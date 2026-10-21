package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

// setupSharedCourse creates a course with two teachers and one published
// block, so that both can start drafts of the same block.
func setupSharedCourse(
	t *testing.T,
	name string,
) (userA, userB TestUser, courseID, blockID uuid.UUID) {
	t.Helper()

	userA = CreateAndLoginUser(
		t, &backendPort, "User", "A", "user-a-"+name, "a-"+name+"@test.com", "password",
	)
	userB = CreateAndLoginUser(
		t, &backendPort, "User", "B", "user-b-"+name, "b-"+name+"@test.com", "password",
	)
	disciplineID := CreateTestDiscipline(t, &backendPort, &userA, "Discipline "+name)
	courseID = CreateTestCourse(t, &backendPort, &userA, disciplineID, name, name)
	InviteTestTeacher(t, &backendPort, &userA, &userB, courseID)

	blockID = CreateTestBlockWithData(
		t, &backendPort, &userA, courseID, nil, `{"text":"base","version":1}`,
	)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))

	return userA, userB, courseID, blockID
}

// TestPublishMergesDrafts checks how a publish reconciles a draft with what
// another teacher published since the draft was started: changes to
// different fields merge, changes to the same content or a deletion against
// a change conflict, and nothing is published on a conflict.
func TestPublishMergesDrafts(t *testing.T) {
	t.Run("same data changed by both conflicts", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "data-data")

		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &userA, courseID, blockID, `{"text":"from A","version":1}`,
		))
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &userB, courseID, blockID, `{"text":"from B","version":1}`,
		))

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))

		status, body := publishCourse(t, &backendPort, &userB, courseID)
		require.Equal(t, http.StatusConflict, status)
		var conflict courses.PublishConflictBody
		require.NoError(t, json.Unmarshal(body, &conflict))
		require.Equal(t, []uuid.UUID{blockID}, conflict.Blocks)

		// Nothing of B's was published, and B's draft is kept
		content := GetCourseContent(t, &backendPort, &userA, courseID)
		require.JSONEq(t, `{"text":"from A","version":1}`, string(content.Blocks[0].Data))
		draftB := GetBlockByID(t, &backendPort, &userB, courseID, blockID)
		require.True(t, draftB.Draft)
		require.JSONEq(t, `{"text":"from B","version":1}`, string(draftB.Data))
		require.Len(t, GetCheckpoints(t, &backendPort, &userA, courseID), 2)

		require.Equal(t, http.StatusNoContent, CancelEdit(t, &backendPort, &userB, courseID))
	})

	t.Run("an edit from a version published over still conflicts", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "stale-seen")

		// A changes and publishes the block while B still looks at version 1
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &userA, courseID, blockID, `{"text":"from A","version":1}`,
		))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, 2, blockVersion(t, &backendPort, &userA, courseID, blockID))

		require.Equal(t, http.StatusOK, PatchTestBlockDataFrom(
			t, &backendPort, &userB, courseID, blockID, 1, `{"text":"from B","version":1}`,
		))
		require.Equal(t, http.StatusConflict, Publish(t, &backendPort, &userB, courseID))
		content := GetCourseContent(t, &backendPort, &userA, courseID)
		require.JSONEq(t, `{"text":"from A","version":1}`, string(content.Blocks[0].Data))

		// B's draft is at its own version now; an edit from version 1 again
		// is a second place editing the same block
		require.Equal(t, http.StatusConflict, PatchTestBlockDataFrom(
			t, &backendPort, &userB, courseID, blockID, 1, `{"text":"again","version":1}`,
		))
		require.Equal(t, http.StatusNotFound, PatchTestBlockDataFrom(
			t, &backendPort, &userA, courseID, blockID, 99, `{"text":"none","version":1}`,
		))
	})

	t.Run("position and data changed apart merge", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "pos-data")
		otherID := CreateTestBlockAfter(t, &backendPort, &userA, courseID, &blockID)
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))

		// A moves the block after the other one, B changes its data
		MoveBlockAfter(t, &backendPort, &userA, courseID, blockID, &otherID)
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &userB, courseID, blockID, `{"text":"from B","version":1}`,
		))

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userB, courseID))

		content := GetCourseContent(t, &backendPort, &userA, courseID)
		require.Equal(t, []uuid.UUID{otherID, blockID}, blockIDs(content.Blocks))
		require.JSONEq(t, `{"text":"from B","version":1}`, string(content.Blocks[1].Data))
	})

	t.Run("position changed by both takes the later publish", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "pos-pos")
		secondID := CreateTestBlockAfter(t, &backendPort, &userA, courseID, &blockID)
		thirdID := CreateTestBlockAfter(t, &backendPort, &userA, courseID, &secondID)
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))

		MoveBlockAfter(t, &backendPort, &userA, courseID, blockID, &secondID)
		MoveBlockAfter(t, &backendPort, &userB, courseID, blockID, &thirdID)

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userB, courseID))

		content := GetCourseContent(t, &backendPort, &userA, courseID)
		require.Equal(t, []uuid.UUID{secondID, thirdID, blockID}, blockIDs(content.Blocks))
	})

	t.Run("deletion against a change conflicts", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "delete-data")

		require.Equal(t, http.StatusNoContent, DeleteTestBlock(
			t, &backendPort, &userA, courseID, blockID,
		))
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &userB, courseID, blockID, `{"text":"from B","version":1}`,
		))

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, http.StatusConflict, Publish(t, &backendPort, &userB, courseID))
		require.Empty(t, GetCourseContent(t, &backendPort, &userA, courseID).Blocks)
	})

	t.Run("deletion by both stays deleted", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "delete-delete")

		require.Equal(t, http.StatusNoContent, DeleteTestBlock(
			t, &backendPort, &userA, courseID, blockID,
		))
		require.Equal(t, http.StatusNoContent, DeleteTestBlock(
			t, &backendPort, &userB, courseID, blockID,
		))

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userB, courseID))
		require.Empty(t, GetCourseContent(t, &backendPort, &userA, courseID).Blocks)
	})

	t.Run("new blocks of both are published", func(t *testing.T) {
		clearDatabases(t)
		userA, userB, courseID, blockID := setupSharedCourse(t, "new-new")

		fromA := CreateTestBlockAfter(t, &backendPort, &userA, courseID, &blockID)
		fromB := CreateTestBlockAfter(t, &backendPort, &userB, courseID, nil)

		// Each teacher sees only their own draft
		require.Equal(t, []uuid.UUID{blockID, fromA}, blockIDs(
			GetDraftBlocks(t, &backendPort, &userA, courseID),
		))
		require.Equal(t, []uuid.UUID{fromB, blockID}, blockIDs(
			GetDraftBlocks(t, &backendPort, &userB, courseID),
		))

		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userA, courseID))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &userB, courseID))

		content := GetCourseContent(t, &backendPort, &userA, courseID)
		require.Equal(t, []uuid.UUID{fromB, blockID, fromA}, blockIDs(content.Blocks))
	})
}
