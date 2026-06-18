package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
)

// Helper to get course content from the active snapshot
func GetCourseContent(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) courses.CourseContentResponse {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/content",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var content courses.CourseContentResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&content))
	return content
}

// Helper to get snapshot blocks
func GetSnapshotBlocks(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	snapshotID uuid.UUID,
) []*blocks.Block {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/snapshots/%s/blocks",
		port.Port(),
		snapshotID,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var snapshotBlocks []*blocks.Block
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&snapshotBlocks))
	return snapshotBlocks
}

// Helper to send heartbeat
func SendHeartbeat(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/heartbeat",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp.StatusCode
}

// Helper to publish draft
func PublishDraft(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
	draftSnapshotID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/publish",
		port.Port(),
		courseID,
	)
	body, _ := json.Marshal(struct {
		DraftSnapshotID uuid.UUID `json:"draftSnapshotID"`
	}{
		DraftSnapshotID: draftSnapshotID,
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp.StatusCode
}

// Helper to cancel edit
func CancelEdit(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/cancel_edit",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp.StatusCode
}

// Helper to switch snapshot
func SwitchSnapshot(
	t *testing.T,
	port *nat.Port,
	testUser *TestUser,
	courseID uuid.UUID,
	targetSnapshotID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/switch_snapshot",
		port.Port(),
		courseID,
	)
	body, _ := json.Marshal(struct {
		TargetSnapshotID uuid.UUID `json:"targetSnapshotID"`
	}{
		TargetSnapshotID: targetSnapshotID,
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	return resp.StatusCode
}

// Test suite for course editing workflow
func TestCourseEditingWorkflow(t *testing.T) {
	clearDatabases(t)

	// --- Setup ---
	teacher := CreateAndLoginUser(
		t,
		&backendPort,
		"Teacher",
		"User",
		"teacher",
		"teacher@test.com",
		"password",
	)
	student := CreateAndLoginUser(
		t,
		&backendPort,
		"Student",
		"User",
		"student",
		"student@test.com",
		"password",
	)
	otherTeacher := CreateAndLoginUser(
		t,
		&backendPort,
		"Other Teacher",
		"User",
		"other_teacher",
		"other_teacher@test.com",
		"password",
	)

	disciplineID := CreateTestDiscipline(t, &backendPort, &teacher, "Test Discipline")
	courseID := CreateTestCourse(t, &backendPort, &teacher, disciplineID, "Test Course")

	// Enroll Student in the course
	{
		inviteURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites", backendPort.Port())
		expiresAt := time.Now().Add(24 * time.Hour)
		inviteBody, _ := json.Marshal(courses.CreateInvite{CourseID: courseID, ProvidedRole: courses.StudentRole, ExpiresAt: &expiresAt})
		req, err := http.NewRequest(http.MethodPost, inviteURL, bytes.NewBuffer(inviteBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := teacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var invite courses.Invite
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&invite))

		joinURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites/%s", backendPort.Port(), invite.ID)
		req, err = http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)
		resp, err = student.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// --- Main Workflow: Lock -> Edit -> Publish ---
	var draftSnapshotID uuid.UUID

	t.Run("1. Lock course and verify draft creation", func(t *testing.T) {
		// A teacher can lock a course to start editing.
		draftSnapshotID = LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		require.NotZero(t, draftSnapshotID)

		// The new draft should contain a copy of the active snapshot's blocks (which is none).
		draftBlocks := GetSnapshotBlocks(t, &backendPort, &teacher, draftSnapshotID)
		require.Len(t, draftBlocks, 0)
	})

	t.Run("2. Another user cannot lock the course", func(t *testing.T) {
		// First, make the other user a teacher for the course.
		// As the original teacher, create an invite.
		inviteURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites", backendPort.Port())
		expiresAt := time.Now().Add(24 * time.Hour)
		inviteBody, _ := json.Marshal(courses.CreateInvite{CourseID: courseID, ProvidedRole: courses.TeacherRole, ExpiresAt: &expiresAt})
		req, err := http.NewRequest(http.MethodPost, inviteURL, bytes.NewBuffer(inviteBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := teacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var invite courses.Invite
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&invite))

		// As the other teacher, join the course.
		joinURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites/%s", backendPort.Port(), invite.ID)
		req, err = http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)
		resp, err = otherTeacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Now, the other teacher tries to lock the course, which should fail.
		lockURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/courses/%s/lock", backendPort.Port(), courseID)
		req, err = http.NewRequest(http.MethodPost, lockURL, nil)
		require.NoError(t, err)
		resp, err = otherTeacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusLocked, resp.StatusCode)
	})

	t.Run("3. Edit blocks within draft", func(t *testing.T) {
		require.NotZero(t, draftSnapshotID, "draftSnapshotID must be set from previous test")

		// Add blocks to the draft.
		block1ID := CreateTestBlock(t, &backendPort, &teacher, draftSnapshotID)
		block2ID := CreateTestBlock(t, &backendPort, &teacher, draftSnapshotID)

		// Verify blocks exist in the draft.
		draftBlocks := GetSnapshotBlocks(t, &backendPort, &teacher, draftSnapshotID)
		require.Len(t, draftBlocks, 2)

		// Update a block in the draft.
		updateURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/snapshots/%s/blocks/%s", backendPort.Port(), draftSnapshotID, block1ID)
		newBlockData := `{"text":"updated content"}`
		body, _ := json.Marshal(blocks.UpdateBlock{Data: json.RawMessage(newBlockData)})
		req, err := http.NewRequest(http.MethodPatch, updateURL, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := teacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		updatedBlock := GetBlockByID(t, &backendPort, &teacher, block1ID)
		require.Equal(t, json.RawMessage(newBlockData), updatedBlock.Data)

		// Delete a block in the draft.
		deleteURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/snapshots/%s/blocks/%s", backendPort.Port(), draftSnapshotID, block2ID)
		req, err = http.NewRequest(http.MethodDelete, deleteURL, nil)
		require.NoError(t, err)
		resp, err = teacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify that the block was soft-deleted from the draft.
		draftBlocks = GetSnapshotBlocks(t, &backendPort, &teacher, draftSnapshotID)
		require.Len(t, draftBlocks, 1)

		// Verify the student still sees the original, unchanged course content.
		studentContent := GetCourseContent(t, &backendPort, &student, courseID)
		require.Len(t, studentContent.Blocks, 0)
	})

	t.Run("4. Publish draft", func(t *testing.T) {
		require.NotZero(t, draftSnapshotID, "draftSnapshotID must be set from previous test")

		// Publish the draft.
		statusCode := PublishDraft(t, &backendPort, &teacher, courseID, draftSnapshotID)
		require.Equal(t, http.StatusNoContent, statusCode)

		// Verify the student now sees the published content.
		studentContent := GetCourseContent(t, &backendPort, &student, courseID)
		require.Equal(t, draftSnapshotID, *studentContent.ActiveSnapshotID)
		require.Len(t, studentContent.Blocks, 1) // One block added, one deleted.
		require.Equal(t, json.RawMessage(`{"text":"updated content"}`), studentContent.Blocks[0].Data)

		// Verify the lock is released and can be re-acquired.
		reLockDraftID := LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		require.NotZero(t, reLockDraftID)
		require.NotEqual(t, draftSnapshotID, reLockDraftID) // Should be a new draft.

		// Clean up by canceling the new edit.
		CancelEdit(t, &backendPort, &teacher, courseID)
	})

	t.Run("Cancel Edit Workflow", func(t *testing.T) {
		// Lock the course to create a draft.
		draftToCancel := LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		CreateTestBlock(t, &backendPort, &teacher, draftToCancel) // Add a block to it.

		// Get the active course content before canceling.
		contentBefore := GetCourseContent(t, &backendPort, &student, courseID)

		// Cancel the edit.
		statusCode := CancelEdit(t, &backendPort, &teacher, courseID)
		require.Equal(t, http.StatusNoContent, statusCode)

		// Verify the active content remains unchanged.
		contentAfter := GetCourseContent(t, &backendPort, &student, courseID)
		require.Equal(t, contentBefore.ActiveSnapshotID, contentAfter.ActiveSnapshotID)
		require.Len(t, contentAfter.Blocks, len(contentBefore.Blocks))

		// Verify the lock is released.
		reLockDraftID := LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		require.NotZero(t, reLockDraftID)
		CancelEdit(t, &backendPort, &teacher, courseID) // Cleanup.
	})

	t.Run("Snapshot Timeline and Switch Workflow", func(t *testing.T) {
		// Publish one more version to have a history.
		draftV2 := LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		CreateTestBlock(t, &backendPort, &teacher, draftV2)
		PublishDraft(t, &backendPort, &teacher, courseID, draftV2)

		// Get snapshot history.
		snapshotsURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/courses/%s/snapshots", backendPort.Port(), courseID)
		req, err := http.NewRequest(http.MethodGet, snapshotsURL, nil)
		require.NoError(t, err)
		resp, err := teacher.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var history []courses.SnapshotMetadataResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&history))
		require.Len(t, history, 3)

		// Find the first snapshot (API returns in reverse chronological order).
		firstSnapshot := history[2]
		require.NotZero(t, firstSnapshot.ID)

		// Lock the course and switch the draft to the first version.
		currentDraft := LockCourse(t, &backendPort, &teacher, courseID).DraftSnapshotID
		statusCode := SwitchSnapshot(t, &backendPort, &teacher, courseID, firstSnapshot.ID)
		require.Equal(t, http.StatusNoContent, statusCode)

		// Verify the draft now has the content of the first snapshot.
		draftBlocks := GetSnapshotBlocks(t, &backendPort, &teacher, currentDraft)
		require.Len(t, draftBlocks, 0) // The very first snapshot was empty.
	})

	t.Run("Optimistic Locking Conflict", func(t *testing.T) {
		// 1. Setup a new course for this specific test.
		conflictCourseID := CreateTestCourse(t, &backendPort, &teacher, disciplineID, "Conflict Course")

		// 2. User A locks the course and gets a draft.
		userADraft := LockCourse(t, &backendPort, &teacher, conflictCourseID).DraftSnapshotID
		CreateTestBlock(t, &backendPort, &teacher, userADraft) // User A makes a change.

		// 3. User B (otherTeacher) also needs to be a teacher on this new course.
		{
			inviteURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites", backendPort.Port())
			expiresAt := time.Now().Add(24 * time.Hour)
			inviteBody, _ := json.Marshal(courses.CreateInvite{CourseID: conflictCourseID, ProvidedRole: courses.TeacherRole, ExpiresAt: &expiresAt})
			req, err := http.NewRequest(http.MethodPost, inviteURL, bytes.NewBuffer(inviteBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			resp, err := teacher.Client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusCreated, resp.StatusCode)
			var invite courses.Invite
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&invite))

			joinURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites/%s", backendPort.Port(), invite.ID)
			req, err = http.NewRequest(http.MethodPost, joinURL, nil)
			require.NoError(t, err)
			resp, err = otherTeacher.Client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		// 4. Simulate User A's lock expiring by having them cancel their edit session.
		CancelEdit(t, &backendPort, &teacher, conflictCourseID)

		// 5. User B now acquires the lock, makes a change, and publishes successfully.
		userBDraft := LockCourse(t, &backendPort, &otherTeacher, conflictCourseID).DraftSnapshotID
		CreateTestBlock(t, &backendPort, &otherTeacher, userBDraft)
		publishStatusCode := PublishDraft(t, &backendPort, &otherTeacher, conflictCourseID, userBDraft)
		require.Equal(t, http.StatusNoContent, publishStatusCode) // B's publish should succeed.

		// 6. User A comes back online and tries to publish their stale draft. This must fail with a conflict.
		publishStatusCode = PublishDraft(t, &backendPort, &teacher, conflictCourseID, userADraft)
		require.Equal(t, http.StatusLocked, publishStatusCode)
	})


}
