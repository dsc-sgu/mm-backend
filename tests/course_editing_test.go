package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	mobynetwork "github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
	"github.com/dsc-sgu/mm-backend/internal/courses"
	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
)

// GetCourseContent returns the course with its published blocks.
func GetCourseContent(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) courses.CourseContentResponse {
	t.Helper()
	return getCourseContent(t, port, testUser, courseID, false)
}

// GetDraftBlocks returns the course's blocks as the teacher sees them in
// their draft: their own draft versions over the published ones.
func GetDraftBlocks(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) []*blocks.Block {
	t.Helper()
	return getCourseContent(t, port, testUser, courseID, true).Blocks
}

func getCourseContent(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
	draft bool,
) courses.CourseContentResponse {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/content?draft=%t",
		port.Port(),
		courseID,
		draft,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var content courses.CourseContentResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&content))
	return content
}

// publishCourse publishes testUser's drafts and returns the response status
// and body.
func publishCourse(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) (int, []byte) {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/publish",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// Publish publishes testUser's drafts and returns the response status. A
// publish that went through must have left the course consistent with its
// latest checkpoint.
func Publish(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) int {
	t.Helper()

	status, _ := publishCourse(t, port, testUser, courseID)
	if status == http.StatusOK {
		requireCheckpointMatchesPublished(t, courseID)
	}
	return status
}

// requireCheckpointMatchesPublished checks the invariant every publish and
// restore keeps: the map of the course's latest checkpoint names exactly
// the block versions the published course is read from. Checkpoints are
// history only, so nothing but this test would notice them drifting.
func requireCheckpointMatchesPublished(t *testing.T, courseID uuid.UUID) {
	t.Helper()

	checkpoint := queryBlockVersions(t, `
		SELECT cb.block_id, cb.version
		FROM checkpoint_blocks cb
		WHERE cb.checkpoint_id = (
			SELECT id FROM checkpoints
			WHERE course_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)`, courseID)
	published := queryBlockVersions(t, `
		SELECT id, version
		FROM (
			SELECT DISTINCT ON (id) id, version, deleted
			FROM blocks
			WHERE course_id = $1 AND NOT draft
			ORDER BY id, version DESC
		) head
		WHERE NOT deleted`, courseID)

	require.ElementsMatch(
		t,
		published,
		checkpoint,
		"the latest checkpoint must map the published block versions",
	)
}

// queryBlockVersions runs a query returning (block id, version) rows for
// courseID and returns them as "id@version" strings.
func queryBlockVersions(t *testing.T, query string, courseID uuid.UUID) []string {
	t.Helper()

	rows, err := testPostgres.Query(query, courseID)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, rows.Close())
	}()

	var versions []string
	for rows.Next() {
		var id string
		var version int
		require.NoError(t, rows.Scan(&id, &version))
		versions = append(versions, fmt.Sprintf("%s@%d", id, version))
	}
	require.NoError(t, rows.Err())
	return versions
}

// CancelEdit discards testUser's drafts and returns the response status.
func CancelEdit(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/cancel-edit",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()

	return resp.StatusCode
}

// GetCheckpoints lists the publishes of the course, latest first.
func GetCheckpoints(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID uuid.UUID,
) []courses.CheckpointResponse {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/checkpoints",
		port.Port(),
		courseID,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list []courses.CheckpointResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	return list
}

// RestoreCheckpoint publishes the course as it was at the checkpoint and
// returns the response status.
func RestoreCheckpoint(
	t *testing.T,
	port *mobynetwork.Port,
	testUser *TestUser,
	courseID, checkpointID uuid.UUID,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/checkpoints/%s/restore",
		port.Port(),
		courseID,
		checkpointID,
	)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	}()

	if resp.StatusCode == http.StatusOK {
		requireCheckpointMatchesPublished(t, courseID)
	}
	return resp.StatusCode
}

func blockIDs(list []*blocks.Block) []uuid.UUID {
	ids := make([]uuid.UUID, len(list))
	for i, b := range list {
		ids[i] = b.ID
	}
	return ids
}

// TestCourseEditingWorkflow walks through editing a course: a teacher's
// changes live in their draft until they publish, a student sees only what
// was published, and every publish is a checkpoint that can be restored.
func TestCourseEditingWorkflow(t *testing.T) {
	clearDatabases(t)

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

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&teacher,
		"Test Discipline",
	)
	courseID := CreateTestCourse(
		t,
		&backendPort,
		&teacher,
		disciplineID,
		"Test Course",
		"Test Course",
	)

	invite, err := createInvite(t, &backendPort, &teacher, courseID, membership.StudentRole)
	require.NoError(t, err)
	require.NoError(t, joinCourse(t, &backendPort, &student, invite.ID))

	var block1ID uuid.UUID

	t.Run("edits stay in the teacher's draft", func(t *testing.T) {
		block1ID = CreateTestBlock(t, &backendPort, &teacher, courseID)
		block2ID := CreateTestBlockAfter(t, &backendPort, &teacher, courseID, &block1ID)

		draft := GetDraftBlocks(t, &backendPort, &teacher, courseID)
		require.Equal(t, []uuid.UUID{block1ID, block2ID}, blockIDs(draft))
		require.True(t, draft[0].Draft)
		require.Equal(t, teacher.ID, draft[0].CreatedBy)

		newData := `{"text":"updated content","version":1}`
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &teacher, courseID, block1ID, newData,
		))
		updated := GetBlockByID(t, &backendPort, &teacher, courseID, block1ID)
		require.JSONEq(t, newData, string(updated.Data))

		require.Equal(t, http.StatusNoContent, DeleteTestBlock(
			t, &backendPort, &teacher, courseID, block2ID,
		))
		draft = GetDraftBlocks(t, &backendPort, &teacher, courseID)
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(draft))

		// Nothing is published yet
		require.Empty(t, GetCourseContent(t, &backendPort, &student, courseID).Blocks)
		require.Empty(t, GetCheckpoints(t, &backendPort, &teacher, courseID))
	})

	t.Run("a student cannot read or edit a draft", func(t *testing.T) {
		url := fmt.Sprintf(
			"http://127.0.0.1:%s/api/v1/courses/%s/content?draft=true",
			backendPort.Port(),
			courseID,
		)
		resp, err := student.Client.Get(url)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusForbidden, resp.StatusCode)

		require.Equal(t, http.StatusForbidden, PatchTestBlockDataFrom(
			t, &backendPort, &student, courseID, block1ID, 1, `{"version":2}`,
		))
	})

	t.Run("publishing makes the draft the course", func(t *testing.T) {
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &teacher, courseID))

		content := GetCourseContent(t, &backendPort, &student, courseID)
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(content.Blocks))
		require.JSONEq(
			t,
			`{"text":"updated content","version":1}`,
			string(content.Blocks[0].Data),
		)
		require.False(t, content.Blocks[0].Draft)

		// The teacher's draft is gone with the publish
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(
			GetDraftBlocks(t, &backendPort, &teacher, courseID),
		))
		require.Equal(t, http.StatusNotFound, Publish(t, &backendPort, &teacher, courseID))

		checkpoints := GetCheckpoints(t, &backendPort, &teacher, courseID)
		require.Len(t, checkpoints, 1)
		require.Equal(t, teacher.ID, checkpoints[0].CreatedBy.ID)
	})

	t.Run("cancelling an edit discards the draft", func(t *testing.T) {
		newID := CreateTestBlock(t, &backendPort, &teacher, courseID)
		require.Equal(t, http.StatusNoContent, DeleteTestBlock(
			t, &backendPort, &teacher, courseID, block1ID,
		))
		require.Equal(t, []uuid.UUID{newID}, blockIDs(
			GetDraftBlocks(t, &backendPort, &teacher, courseID),
		))

		require.Equal(t, http.StatusNoContent, CancelEdit(t, &backendPort, &teacher, courseID))

		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(
			GetDraftBlocks(t, &backendPort, &teacher, courseID),
		))
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(
			GetCourseContent(t, &backendPort, &student, courseID).Blocks,
		))
	})

	t.Run("a checkpoint can be restored", func(t *testing.T) {
		block3ID := CreateTestBlockAfter(t, &backendPort, &teacher, courseID, &block1ID)
		require.Equal(t, http.StatusOK, PatchTestBlockData(
			t, &backendPort, &teacher, courseID, block1ID, `{"text":"second","version":1}`,
		))
		require.Equal(t, http.StatusOK, Publish(t, &backendPort, &teacher, courseID))
		require.Equal(t, []uuid.UUID{block1ID, block3ID}, blockIDs(
			GetCourseContent(t, &backendPort, &student, courseID).Blocks,
		))

		checkpoints := GetCheckpoints(t, &backendPort, &teacher, courseID)
		require.Len(t, checkpoints, 2)
		first := checkpoints[1]

		url := fmt.Sprintf(
			"http://127.0.0.1:%s/api/v1/courses/%s/checkpoints/%s/blocks",
			backendPort.Port(),
			courseID,
			first.ID,
		)
		resp, err := teacher.Client.Get(url)
		require.NoError(t, err)
		var firstBlocks []*blocks.Block
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&firstBlocks))
		require.NoError(t, resp.Body.Close())
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(firstBlocks))
		require.JSONEq(
			t,
			`{"text":"updated content","version":1}`,
			string(firstBlocks[0].Data),
		)

		require.Equal(t, http.StatusOK, RestoreCheckpoint(
			t, &backendPort, &teacher, courseID, first.ID,
		))

		content := GetCourseContent(t, &backendPort, &student, courseID)
		require.Equal(t, []uuid.UUID{block1ID}, blockIDs(content.Blocks))
		require.JSONEq(
			t,
			`{"text":"updated content","version":1}`,
			string(content.Blocks[0].Data),
		)
		require.Len(t, GetCheckpoints(t, &backendPort, &teacher, courseID), 3)
	})
}
