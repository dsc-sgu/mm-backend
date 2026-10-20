package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
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

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	require.NotZero(t, groupID)
}

func TestGetTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	require.NotZero(t, groupID)

	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
	require.NotZero(t, taskID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
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
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
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

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
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

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
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

// TestDeleteTaskGroupWithTasks is a regression test for a bug where deleting
// a task group that still had tasks referencing it (from any snapshot
// generation) crashed with a raw foreign key violation instead of succeeding
// via soft-delete.
func TestDeleteTaskGroupWithTasks(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
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

// TestCreateTaskGroupReusesDeletedName checks that a task group name freed up
// by a soft-delete can be reused: uniqueness is scoped to non-deleted groups
// only (a partial index), not to the table as a whole.
func TestCreateTaskGroupReusesDeletedName(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	firstGroupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	deleteURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
		backendPort.Port(),
		firstGroupID,
	)
	deleteReq, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	require.NoError(t, err)
	deleteResp, err := testUser.Client.Do(deleteReq)
	require.NoError(t, err)
	defer func() {
		if err := deleteResp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	secondGroupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	require.NotZero(t, secondGroupID)
	require.NotEqual(t, firstGroupID, secondGroupID)
}

func TestCreateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
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

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)

	body, err := json.Marshal(blocks.CreateBlockCommand{
		BlockType: "task",
		Task: &blocks.TaskData{
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

// TestCreateTaskBlockRejectsData checks that a task block takes no content:
// its fields come in "task", and data is only for text and quiz blocks.
func TestCreateTaskBlockRejectsData(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)
	body, err := json.Marshal(blocks.CreateBlockCommand{
		BlockType: "task",
		Data:      []byte("true"),
		Task: &blocks.TaskData{
			TaskGroupID: groupID,
			Name:        "Task 1",
			MaxGrade:    100,
			MaxAttempts: 5,
		},
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
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetTasks(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)

	taskID1 := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
	taskID2 := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 2",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s/tasks",
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
// old, no-longer-current block id, silently orphaning it. The copied block
// must link to the same task.
func TestTaskSurvivesPublishAndReEdit(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, draftSnapshotID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)

	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, draftSnapshotID,
	))

	// Re-opening the course for editing copies the now-active snapshot's
	// blocks into a fresh draft — this is where the bug used to lose the
	// task's metadata.
	lockResult := LockCourse(t, &backendPort, &testUser, courseID)
	require.NotEqual(t, uuid.Nil, lockResult.DraftSnapshotID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s/tasks",
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
	require.Equal(
		t,
		taskID,
		receivedTasks[0].ID,
		"task should keep its id in the new draft",
	)
	require.Equal(t, "Task 1", receivedTasks[0].Name)
	require.Equal(t, groupID, receivedTasks[0].TaskGroupID)
}

// patchTestTask sends PATCH /tasks/{task_id} and returns the response status
// and the decoded task.
func patchTestTask(
	t *testing.T,
	testUser *TestUser,
	taskID uuid.UUID,
	update tasks.TaskUpdate,
) (int, *tasks.Task) {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s",
		backendPort.Port(),
		taskID,
	)
	body, err := json.Marshal(update)
	require.NoError(t, err)

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

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	var task tasks.Task
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&task))
	return resp.StatusCode, &task
}

// TestUpdateTask patches a published task through PATCH /tasks/{task_id}.
// A task lives outside snapshots, so no course edit lock is held.
func TestUpdateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, draftSnapshotID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
	// Publishing releases the course edit lock.
	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, draftSnapshotID,
	))

	newGrade := float32(50)
	statement := json.RawMessage(`{"text":"Write a parser"}`)
	status, updated := patchTestTask(t, &testUser, taskID, tasks.TaskUpdate{
		Statement: statement,
		MaxGrade:  &newGrade,
		Patterns:  &[]string{"*.go"},
	})
	require.Equal(t, http.StatusOK, status)

	require.Equal(t, taskID, updated.ID)
	require.InDelta(t, newGrade, updated.MaxGrade, 0.001)
	require.Equal(t, []string{"*.go"}, []string(updated.Patterns))
	require.NotNil(t, updated.Statement)
	require.JSONEq(t, string(statement), string(*updated.Statement))
}

// TestAttemptsSurviveRepublish checks that attempts made before a course is
// edited and published again stay with the task.
func TestAttemptsSurviveRepublish(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, draftSnapshotID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, draftSnapshotID,
	))

	files := map[string]string{"solution.go": "package main\n"}
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, files,
	))

	lockResult := LockCourse(t, &backendPort, &testUser, courseID)
	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, lockResult.DraftSnapshotID,
	))

	files["solution.go"] = "package main\n\nfunc main() {}\n"
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, files,
	))

	attempts := GetTestAttempts(t, &backendPort, &testUser, taskID, testUser.ID)
	require.Len(t, attempts, 2)
}

// TestCancelEditDeletesDraftTask checks that a task created in a draft that
// is then discarded is deleted with it.
func TestCancelEditDeletesDraftTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)

	require.Equal(t, http.StatusNoContent, CancelEdit(
		t, &backendPort, &testUser, courseID,
	))

	newGrade := float32(50)
	status, _ := patchTestTask(t, &testUser, taskID, tasks.TaskUpdate{
		MaxGrade: &newGrade,
	})
	require.Equal(t, http.StatusNotFound, status)
}

// TestCreateTaskRejectsTakenName checks that two tasks of one group in one
// snapshot cannot share a name, which a git push uses to pick the task.
func TestCreateTaskRejectsTakenName(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)
	body, err := json.Marshal(blocks.CreateBlockCommand{
		BlockType: "task",
		Task: &blocks.TaskData{
			TaskGroupID: groupID,
			Name:        "Task 1",
			MaxGrade:    100,
			MaxAttempts: 5,
		},
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
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestUploadTemplate(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	_ = CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	UploadTestTemplate(t, &backendPort, &testUser, groupID, map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "template readme",
	})
}

func TestUploadTemplateRequiresLock(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	_ = CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")
	require.Equal(t, http.StatusNoContent, CancelEdit(t, &backendPort, &testUser, courseID))

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s/template",
		backendPort.Port(),
		groupID,
	)
	require.Equal(t, http.StatusLocked, postTestZip(t, &testUser, url, map[string]string{
		"README.md": "template readme",
	}))
}

func TestUploadTemplateEmptyBody(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID, _ := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	_ = CreateTestTask(t, &backendPort, &testUser, courseID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s/template",
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

	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
	)
	taskID := CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"Task 1",
	)
	// Note: the draft created by setupCourseForTasks is never published.

	require.NotEqual(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, map[string]string{
			"solution.go": "package main\n\nfunc main() {}\n",
		},
	))
}
