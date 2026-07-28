package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/tasks"
)

func setupCourseForTasks(
	t *testing.T,
) (userID, disciplineID, courseID uuid.UUID) {
	t.Helper()

	userID = CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID = CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID = CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
		"Test Course",
	)
	require.NotZero(t, courseID)

	return userID, disciplineID, courseID
}

func TestCreateTaskGroup(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	require.NotZero(t, groupID)
}

func TestGetTaskGroup(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	require.NotZero(t, groupID)

	taskID := CreateTestTask(t, &backendPort, userID, groupID, "Task 1")
	require.NotZero(t, taskID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
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

	userID, _, _ := setupCourseForTasks(t)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		uuid.New(),
		userID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
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

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	newName := "Renamed Group"
	body, _ := json.Marshal(tasks.UpdateTaskGroup{Name: &newName})

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
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

	getResp, err := http.DefaultClient.Do(getReq)
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

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")

	taskID := CreateTestTask(t, &backendPort, userID, groupID, "Task 1")
	require.NotZero(t, taskID)
}

// TestCreateTaskWithoutPatterns is a regression test for a bug where creating
// a task without an explicit "patterns" field crashed with a 500: pq.StringArray
// marshals a nil slice to SQL NULL, which violated the tasks.patterns
// NOT NULL constraint (the column's DEFAULT '{}' only applies when the column
// is omitted from the INSERT, not when an explicit NULL is passed).
func TestCreateTaskWithoutPatterns(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	body, err := json.Marshal(tasks.CreateTask{
		Name:        "Task NoPatternsField",
		Data:        []byte("true"),
		MaxGrade:    100,
		MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.NotContains(t, string(body), "patterns")

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created tasks.CreateTaskResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.NotZero(t, created.ID)
}

func TestGetTasks(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")

	taskID1 := CreateTestTask(t, &backendPort, userID, groupID, "Task 1")
	taskID2 := CreateTestTask(t, &backendPort, userID, groupID, "Task 2")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
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

func TestUpdateTask(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	taskID := CreateTestTask(t, &backendPort, userID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		taskID,
		userID,
	)

	newGrade := float32(50)
	body, _ := json.Marshal(tasks.UpdateTask{
		MaxGrade: &newGrade,
		Patterns: &[]string{"*.go"},
	})

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated tasks.Task
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))

	require.Equal(t, taskID, updated.ID)
	require.InDelta(t, newGrade, updated.MaxGrade, 0.001)
	require.Equal(t, []string{"*.go"}, []string(updated.Patterns))
}

func TestDeleteTask(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	taskID1 := CreateTestTask(t, &backendPort, userID, groupID, "Task 1")
	_ = CreateTestTask(t, &backendPort, userID, groupID, "Task 2")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		taskID1,
		userID,
	)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	getTasksURL := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	getReq, err := http.NewRequest(http.MethodGet, getTasksURL, nil)
	require.NoError(t, err)

	getResp, err := http.DefaultClient.Do(getReq)
	require.NoError(t, err)
	defer func() {
		if err := getResp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var remaining []*tasks.Task
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&remaining))
	require.Len(t, remaining, 1)
	require.Equal(t, "Task 2", remaining[0].Name)
}

// TestDeleteLastTaskFails locks in the service-level guard in
// tasks.Service.DeleteTask: a task group must always keep at least one task,
// since a group with zero tasks would still expose an active git repo/name
// with nothing to submit against.
func TestDeleteLastTaskFails(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	taskID := CreateTestTask(t, &backendPort, userID, groupID, "Only Task")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/tasks/%s?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		taskID,
		userID,
	)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestUploadTemplate(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	_ = CreateTestTask(t, &backendPort, userID, groupID, "Task 1")

	zipData := buildTestZip(t, map[string]string{
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "template readme",
	})

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/template?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(zipData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestUploadTemplateEmptyBody(t *testing.T) {
	clearDatabases(t)

	userID, _, courseID := setupCourseForTasks(t)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "Group 1")
	_ = CreateTestTask(t, &backendPort, userID, groupID, "Task 1")

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/tasks/%s/template?fake_user_id=%s",
		backendPort.Port(),
		groupID,
		userID,
	)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(nil))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
