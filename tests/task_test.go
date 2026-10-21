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
) (testUser TestUser, disciplineID, courseID uuid.UUID) {
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

	return testUser, disciplineID, courseID
}

// postTestTaskBlock sends a task block creation request and returns the
// response status.
func postTestTaskBlock(
	t *testing.T,
	testUser *TestUser,
	courseID uuid.UUID,
	task blocks.TaskData,
) int {
	t.Helper()

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)
	body, err := json.Marshal(blocks.CreateBlockCommand{
		BlockType: "task",
		Task:      &task,
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
	return resp.StatusCode
}

// TestCreateTaskRequiresOneTaskGroup checks that a new task names either an
// existing group or a new one, not both and not neither.
func TestCreateTaskRequiresOneTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	for name, group := range map[string]blocks.TaskGroupRef{
		"neither": {},
		"both": {
			ID:  &groupID,
			New: &blocks.NewTaskGroup{Name: "Group 2"},
		},
		"empty new name": {New: &blocks.NewTaskGroup{}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, http.StatusBadRequest, postTestTaskBlock(
				t, &testUser, courseID, blocks.TaskData{
					TaskGroup: group,
					Name:      "Task 2",
					MaxGrade:  100,
				},
			))
		})
	}
}

// TestCreateTaskRejectsTakenTaskGroupName checks that a new task group cannot
// take the name of a live group of the course.
func TestCreateTaskRejectsTakenTaskGroupName(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	require.Equal(t, http.StatusConflict, postTestTaskBlock(
		t, &testUser, courseID, blocks.TaskData{
			TaskGroup: blocks.TaskGroupRef{
				New: &blocks.NewTaskGroup{Name: "Group 1"},
			},
			Name:     "Task 2",
			MaxGrade: 100,
		},
	))
}

func TestGetTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
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

	testUser, _, _ := setupCourseForTasks(t)

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

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/task-groups/%s",
		backendPort.Port(),
		groupID,
	)

	newName := "Renamed Group"
	body, _ := json.Marshal(tasks.UpdateTaskGroup{Name: newName})

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

// TestCancelEditDeletesDraftTaskGroup checks that a task group created in a
// draft is deleted together with its last task when the draft is discarded.
func TestCancelEditDeletesDraftTaskGroup(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.Equal(t, http.StatusNoContent, CancelEdit(
		t, &backendPort, &testUser, courseID,
	))

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
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestCreateTaskGroupReusesDeletedName checks that a task group name freed up
// by a soft-delete can be reused: uniqueness is scoped to non-deleted groups
// only (a partial index), not to the table as a whole.
func TestCreateTaskGroupReusesDeletedName(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	firstGroupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.Equal(t, http.StatusNoContent, CancelEdit(
		t, &backendPort, &testUser, courseID,
	))

	secondGroupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.NotEqual(t, firstGroupID, secondGroupID)
}

func TestCreateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
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

	testUser, _, courseID := setupCourseForTasks(t)

	task := blocks.TaskData{
		TaskGroup: blocks.TaskGroupRef{
			New: &blocks.NewTaskGroup{Name: "Group 1"},
		},
		Name:     "Task NoPatternsField",
		MaxGrade: 100,
	}
	body, err := json.Marshal(task)
	require.NoError(t, err)
	require.NotContains(t, string(body), "patterns")

	require.Equal(t, http.StatusCreated, postTestTaskBlock(
		t, &testUser, courseID, task,
	))
}

// TestCreateTaskBlockRejectsData checks that a task block takes no content:
// its fields come in "task", and data is only for the other blocks.
func TestCreateTaskBlockRejectsData(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s/blocks",
		backendPort.Port(),
		courseID,
	)
	body, err := json.Marshal(blocks.CreateBlockCommand{
		BlockType: "task",
		Data:      []byte("true"),
		Task: &blocks.TaskData{
			TaskGroup: blocks.TaskGroupRef{
				New: &blocks.NewTaskGroup{Name: "Group 1"},
			},
			Name:     "Task 1",
			MaxGrade: 100,
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

// TestTaskBlockDataReferencesTask checks that a task block's data reads as a
// reference to its task.
func TestTaskBlockDataReferencesTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	content := GetCourseContent(t, &backendPort, &testUser, courseID)
	require.Len(t, content.Blocks, 1)
	require.Equal(t, "task", content.Blocks[0].BlockType)
	require.JSONEq(
		t,
		fmt.Sprintf(`{"version":1,"taskId":"%s"}`, taskID),
		string(content.Blocks[0].Data),
	)
}

func TestGetTasks(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, taskID1 := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
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

// TestTaskSurvivesPublishAndReEdit checks that a task keeps its id while
// its block is edited again after being published: a draft version of the
// block links to the same task.
func TestTaskSurvivesPublishAndReEdit(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	// Moving the task block gives it a draft version
	content := GetCourseContent(t, &backendPort, &testUser, courseID)
	require.Len(t, content.Blocks, 1)
	MoveBlockAfter(t, &backendPort, &testUser, courseID, content.Blocks[0].ID, nil)
	draft := GetBlockByID(t, &backendPort, &testUser, courseID, content.Blocks[0].ID)
	require.True(t, draft.Draft)
	require.JSONEq(
		t,
		fmt.Sprintf(`{"version":1,"taskId":"%s"}`, taskID),
		string(draft.Data),
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

	require.Len(t, receivedTasks, 1)
	require.Equal(
		t,
		taskID,
		receivedTasks[0].ID,
		"task should keep its id in the draft version",
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
// A task lives outside block versions, so the change applies at once.
func TestUpdateTask(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

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

// TestPushAttemptRespectsMaxAttempts checks that a task takes no more
// attempts than its limit, that a submission rejected by the patterns does
// not use one up, and that clearing the limit with null lifts it.
func TestPushAttemptRespectsMaxAttempts(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
		"*.go",
	)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	status, _ := patchTestTask(t, &testUser, taskID, tasks.TaskUpdate{
		MaxAttempts: tasks.OmittableNullable[int]{Sent: true, Value: 0},
	})
	require.Equal(t, http.StatusUnprocessableEntity, status)

	status, updated := patchTestTask(t, &testUser, taskID, tasks.TaskUpdate{
		MaxAttempts: tasks.OmittableNullable[int]{Sent: true, Value: 1},
	})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 1, *updated.MaxAttempts)

	require.Equal(t, http.StatusBadRequest, PushTestAttempt(
		t, &backendPort, &testUser, taskID, map[string]string{"notes.txt": "x"},
	))
	solution := map[string]string{"solution.go": "package main\n"}
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, solution,
	))
	require.Equal(t, http.StatusConflict, PushTestAttempt(
		t, &backendPort, &testUser, taskID, solution,
	))

	status, updated = patchTestTask(t, &testUser, taskID, tasks.TaskUpdate{
		MaxAttempts: tasks.OmittableNullable[int]{Sent: true, Null: true},
	})
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, updated.MaxAttempts)
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, solution,
	))
}

// TestAttemptsSurviveRepublish checks that attempts made before a course is
// edited and published again stay with the task.
func TestAttemptsSurviveRepublish(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

	files := map[string]string{"solution.go": "package main\n"}
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, files,
	))

	// Edit the course again: a new block and a new version of the task block
	CreateTestBlock(t, &backendPort, &testUser, courseID)
	content := GetCourseContent(t, &backendPort, &testUser, courseID)
	MoveBlockAfter(t, &backendPort, &testUser, courseID, content.Blocks[0].ID, nil)
	require.Equal(t, http.StatusOK, Publish(t, &backendPort, &testUser, courseID))

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

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
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

// TestCreateTaskRejectsTakenName checks that two tasks of one group cannot
// share a name, which a git push uses to pick the task.
func TestCreateTaskRejectsTakenName(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	require.Equal(t, http.StatusConflict, postTestTaskBlock(
		t, &testUser, courseID, blocks.TaskData{
			TaskGroup: blocks.TaskGroupRef{ID: &groupID},
			Name:      "Task 1",
			MaxGrade:  100,
		},
	))
}

func TestUploadTemplate(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

	UploadTestTemplate(t, &backendPort, &testUser, groupID, map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "template readme",
	})
}

func TestUploadTemplateEmptyBody(t *testing.T) {
	clearDatabases(t)

	testUser, _, courseID := setupCourseForTasks(t)

	groupID, _ := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)

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

	testUser, _, courseID := setupCourseForTasks(t)

	_, taskID := CreateTestTaskInNewGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"Group 1",
		"Task 1",
	)
	// Note: the task block is never published.

	require.NotEqual(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, map[string]string{
			"solution.go": "package main\n\nfunc main() {}\n",
		},
	))
}
