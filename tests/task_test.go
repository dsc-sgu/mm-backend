package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/content"
	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

func setupCourseForTasks(
	t *testing.T,
) (testUser TestUser, disciplineID, courseID, draftSnapshotID uuid.UUID) {
	t.Helper()

	testUser = CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID = CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID = CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
		"Test Course",
	)
	require.NotZero(t, courseID)

	// Creating a task creates a "task"-typed block, which requires a draft
	// snapshot to exist for the course.
	lockResult := LockCourse(t, &backendPort, &testUser, courseID)

	return testUser, disciplineID, courseID, lockResult.DraftSnapshotID
}

func TestCreateTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	require.NotZero(t, groupID)
}

func TestGetTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	require.NotZero(t, groupID)

	taskID := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")
	require.NotZero(t, taskID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s",
		backendPort.Port(),
		groupID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var group tasks.TaskGroupWithTasks
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&group))

	require.Equal(t, groupID, group.ID)
	require.Equal(t, courseID, group.CourseID)
	require.Equal(t, "Group 1", group.Name)
	require.Len(t, group.Tasks, 1)
	require.Equal(t, taskID, group.Tasks[0].ID)
}

func TestGetTaskGroupNotFound(t *testing.T) {
	clearDatabases(t)

	testUser, _, _, _ := setupCourseForTasks(t)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s",
		backendPort.Port(),
		uuid.New(),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s",
		backendPort.Port(),
		groupID,
	)

	newName := "Renamed Group"
	body, _ := json.Marshal(tasks.UpdateTaskGroup{Name: &newName})

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

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated tasks.TaskGroup
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))

	require.Equal(t, groupID, updated.ID)
	require.Equal(t, newName, updated.Name)
}

func TestDeleteTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s",
		backendPort.Port(),
		groupID,
	)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Group should no longer be retrievable.
	getReq, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	getResp, err := testUser.Client.Do(getReq)
	require.NoError(t, err)
	defer func() {
		if err := getResp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestCreateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")

	taskID := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")
	require.NotZero(t, taskID)
}

// TestCreateTaskWithoutPatterns is a regression test for a bug where creating
// a task without an explicit "patterns" field crashed with a 500: pq.StringArray
// marshals a nil slice to SQL NULL, which violated the tasks.patterns
// NOT NULL constraint (the column's DEFAULT '{}' only applies when the column
// is omitted from the INSERT, not when an explicit NULL is passed).
func TestCreateTaskWithoutPatterns(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)

	body, err := json.Marshal(content.CreateBlockCommand{
		BlockType: "task",
		Data:      []byte("true"),
		Task: &content.TaskData{
			TaskGroupID: groupID,
			Name:        "Task NoPatternsField",
			MaxGrade:    100,
			MaxAttempts: 5,
		},
	})
	require.NoError(t, err)
	require.NotContains(t, string(body), "patterns")

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

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.NotZero(t, created.ID)
}

func TestGetTasks(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")

	taskID1 := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")
	taskID2 := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 2")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks",
		backendPort.Port(),
		groupID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var receivedTasks []*tasks.Task
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&receivedTasks))

	require.Len(t, receivedTasks, 2)
	require.Equal(t, taskID1, receivedTasks[0].ID)
	require.Equal(t, taskID2, receivedTasks[1].ID)
}

// TestTaskSurvivesPublishAndReEdit is a regression test for a bug where
// copying blocks to a new snapshot (opening a course for editing, or
// switching snapshots) left a task-type block's metadata behind under its
// old, no-longer-current block id, silently orphaning it. Each snapshot
// generation of a task must carry its own, fully populated task data.
func TestTaskSurvivesPublishAndReEdit(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, draftSnapshotID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	taskID := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, draftSnapshotID,
	))

	// Re-opening the course for editing copies the now-active snapshot's
	// blocks into a fresh draft — this is where the bug used to lose the
	// task's metadata.
	lockResult := LockCourse(t, &backendPort, &testUser, courseID)
	require.NotEqual(t, uuid.Nil, lockResult.DraftSnapshotID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks",
		backendPort.Port(),
		groupID,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var receivedTasks []*tasks.Task
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&receivedTasks))

	require.Len(t, receivedTasks, 1)
	require.NotEqual(t, taskID, receivedTasks[0].ID, "task should have a new block id in the new draft generation")
	require.Equal(t, "Task 1", receivedTasks[0].Name)
	require.Equal(t, groupID, receivedTasks[0].TaskGroupID)
}

// TestUpdateTask exercises task field updates through the same course-editing
// path as any other block patch (task data is just a block subtype), rather
// than a standalone /tasks endpoint.
func TestUpdateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, draftSnapshotID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	taskID := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/snapshots/%s/blocks/%s",
		backendPort.Port(),
		courseID,
		draftSnapshotID,
		taskID,
	)

	newGrade := float32(50)
	body, _ := json.Marshal(content.PatchBlockCommand{
		Task: &content.TaskUpdate{
			MaxGrade: &newGrade,
			Patterns: &[]string{"*.go"},
		},
	})

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

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated struct {
		Block *struct {
			ID uuid.UUID `json:"id"`
		} `json:"block"`
		Task *content.TaskData `json:"task"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))

	require.Equal(t, taskID, updated.Block.ID)
	require.NotNil(t, updated.Task)
	require.InDelta(t, newGrade, updated.Task.MaxGrade, 0.001)
	require.Equal(t, []string{"*.go"}, updated.Task.Patterns)
}

func TestUploadTemplate(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	_ = CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	zipData := buildTestZip(t, map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "template readme",
	})

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/template",
		backendPort.Port(),
		groupID,
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(zipData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/zip")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestUploadTemplateEmptyBody(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	_ = CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/template",
		backendPort.Port(),
		groupID,
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(nil))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/zip")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestPushAttemptRejectsUnpublishedTask is a regression test for attempts
// respecting course-editing's draft/published model: a task that only exists
// in an unpublished draft must not be attemptable yet.
func TestPushAttemptRejectsUnpublishedTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, &testUser, courseID, "Group 1")
	taskID := CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")
	// Note: the draft created by setupCourseForTasks is never published.

	zipData := buildTestZip(t, map[string]string{
		"solution.go": "package main\n\nfunc main() {}\n",
	})

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/attempts?taskID=%s",
		backendPort.Port(),
		taskID,
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(zipData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/zip")

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.NotEqual(t, http.StatusCreated, resp.StatusCode)
}
