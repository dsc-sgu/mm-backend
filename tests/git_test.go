package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
)

func TestGitSubmitViaSSHTagPush(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test",
		"Student",
		"gitstudent",
		"gitstudent@test.com",
		"password",
	)
	require.NotZero(t, userID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Git Discipline",
	)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"gitcourse",
		"Git Course",
	)

	groupID := CreateTestTaskGroup(t, &backendPort, userID, courseID, "group1")
	taskID := CreateTestTask(t, &backendPort, userID, groupID, "taskA")

	identity := generateSSHKeyPair(t)
	RegisterTestSSHKey(t, &backendPort, userID, "test-key", identity.authorizedKey)

	workDir := t.TempDir()
	repoURL := fmt.Sprintf(
		"ssh://git@127.0.0.1:%s/gitcourse/group1.git",
		backendSSHPort.Port(),
	)

	out, err := runGitCommand(t, workDir, identity, "clone", repoURL, "repo")
	require.NoError(t, err, out)

	repoDir := filepath.Join(workDir, "repo")

	require.NoError(t, os.WriteFile(
		filepath.Join(repoDir, "solution.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	))

	out, err = runGitCommand(t, repoDir, identity, "add", "solution.go")
	require.NoError(t, err, out)

	out, err = runGitCommand(t, repoDir, identity, "commit", "-m", "solution")
	require.NoError(t, err, out)

	out, err = runGitCommand(t, repoDir, identity, "tag", "submission-1")
	require.NoError(t, err, out)

	out, err = runGitCommand(
		t, repoDir, identity,
		"push", "origin", "submission-1", "-o", "submit=taskA",
	)
	require.NoError(t, err, out)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/attempts/%s/%s?fake_user_id=%s",
		backendPort.Port(),
		taskID,
		userID,
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

	var receivedAttempts []attempt.Attempt
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&receivedAttempts))

	require.Len(t, receivedAttempts, 1)
	require.Equal(t, taskID, receivedAttempts[0].TaskID)
	require.Equal(t, userID, receivedAttempts[0].UserID)
}
