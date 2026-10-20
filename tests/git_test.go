package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/auth/sshkeys"
)

// gitRepoPath is the SSH path of the repository setupGitTask creates.
const gitRepoPath = "gitcourse/group1.git"

// setupGitTask creates a published task "taskA" in group "group1" of course
// "gitcourse", owned by a user with a registered SSH key. A non-nil template
// is uploaded to the group before publishing.
func setupGitTask(
	t *testing.T,
	template map[string]string,
	patterns ...string,
) (testUser TestUser, taskID uuid.UUID, identity sshIdentity) {
	t.Helper()

	testUser = CreateAndLoginUser(
		t,
		&backendPort,
		"Test",
		"Student",
		"gitstudent",
		"gitstudent@test.com",
		"password",
	)
	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Git Discipline",
	)
	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"gitcourse",
		"Git Course",
	)
	groupID := CreateTestTaskGroup(
		t,
		&backendPort,
		&testUser,
		courseID,
		"group1",
	)

	// Creating a task creates a "task"-typed block, which requires a draft
	// snapshot to exist for the course.
	lockResult := LockCourse(t, &backendPort, &testUser, courseID)

	taskID = CreateTestTask(
		t,
		&backendPort,
		&testUser,
		courseID,
		groupID,
		"taskA",
		patterns...,
	)
	if template != nil {
		UploadTestTemplate(t, &backendPort, &testUser, groupID, template)
	}

	// A task only becomes attemptable (over HTTP or SSH) once it is part of
	// the course's active, published snapshot.
	require.Equal(t, http.StatusNoContent, PublishDraft(
		t, &backendPort, &testUser, courseID, lockResult.DraftSnapshotID,
	))

	identity = generateSSHKeyPair(t)
	RegisterTestSSHKey(
		t,
		&backendPort,
		&testUser,
		"test-key",
		identity.authorizedKey,
	)

	return testUser, taskID, identity
}

func TestGitSubmitViaSSHTagPush(t *testing.T) {
	clearDatabases(t)

	testUser, taskID, identity := setupGitTask(t, nil)

	repoDir := CloneTestRepo(t, &backendSSHPort, identity, gitRepoPath)
	require.NoError(t, os.WriteFile(
		filepath.Join(repoDir, "solution.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	))
	runGit(t, repoDir, identity, "add", "solution.go")
	runGit(t, repoDir, identity, "commit", "-m", "solution")
	runGit(t, repoDir, identity, "tag", "submission-1")
	runGit(t, repoDir, identity, "push", "origin", "submission-1", "-o", "submit=taskA")

	attempts := GetTestAttempts(t, &backendPort, &testUser, taskID, testUser.ID)
	require.Len(t, attempts, 1)
	require.Equal(t, taskID, attempts[0].TaskID)
	require.Equal(t, testUser.ID, attempts[0].UserID)
}

// TestPushAttemptReplacesPatternFiles checks which files a web attempt leaves
// in the repository: every file from the zip is added, and of the files
// already there only those matching the task's patterns are removed.
func TestPushAttemptReplacesPatternFiles(t *testing.T) {
	clearDatabases(t)

	testUser, taskID, identity := setupGitTask(t, map[string]string{
		"README.md": "template readme",
		"a/stub.go": "package a\n",
	}, "a/*")

	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, map[string]string{
			"a/x.go":    "package a\n",
			"notes.txt": "draft",
		},
	))
	require.Equal(t, http.StatusCreated, PushTestAttempt(
		t, &backendPort, &testUser, taskID, map[string]string{
			"a/y.go": "package a\n",
		},
	))

	// a/stub.go (template) and a/x.go (first attempt) match "a/*", so they are
	// gone; README.md and notes.txt match no pattern, so they stay.
	repoDir := CloneTestRepo(t, &backendSSHPort, identity, gitRepoPath)
	files := strings.Fields(runGit(t, repoDir, identity, "ls-files"))
	require.Equal(t, []string{"README.md", "a/y.go", "notes.txt"}, files)
	require.Len(t, GetTestAttempts(t, &backendPort, &testUser, taskID, testUser.ID), 2)
}

func TestPushAttemptRejectsBadArchive(t *testing.T) {
	clearDatabases(t)

	testUser, taskID, _ := setupGitTask(t, nil, "a/*")

	for name, files := range map[string]map[string]string{
		"empty":       {},
		"no matching": {"notes.txt": "draft"},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, http.StatusBadRequest, PushTestAttempt(
				t, &backendPort, &testUser, taskID, files,
			))
		})
	}
	require.Empty(t, GetTestAttempts(t, &backendPort, &testUser, taskID, testUser.ID))
}

// TestPushAttemptSameArchiveTwice is a regression test for a resubmission
// that leaves the tree unchanged: it must still be recorded as a new attempt
// rather than fail on an empty commit.
func TestPushAttemptSameArchiveTwice(t *testing.T) {
	clearDatabases(t)

	testUser, taskID, _ := setupGitTask(t, nil, "a/*")

	files := map[string]string{"a/x.go": "package a\n"}
	for range 2 {
		require.Equal(t, http.StatusCreated, PushTestAttempt(
			t, &backendPort, &testUser, taskID, files,
		))
	}
	require.Len(t, GetTestAttempts(t, &backendPort, &testUser, taskID, testUser.ID), 2)
}

// TestSSHKeyCannotBeRegisteredTwice is a regression test for SSH identity
// hijacking: a public key is public, so if a second user could register
// someone else's key, that owner's pushes could resolve to the second user.
func TestSSHKeyCannotBeRegisteredTwice(t *testing.T) {
	clearDatabases(t)

	owner := CreateAndLoginUser(
		t, &backendPort, "Key", "Owner", "keyowner", "keyowner@test.com", "password",
	)
	other := CreateAndLoginUser(
		t, &backendPort, "Key", "Thief", "keythief", "keythief@test.com", "password",
	)

	identity := generateSSHKeyPair(t)
	RegisterTestSSHKey(t, &backendPort, &owner, "owner-key", identity.authorizedKey)

	for _, user := range []*TestUser{&other, &owner} {
		body, err := json.Marshal(sshkeys.AddSSHKey{Name: "dup", Key: identity.authorizedKey})
		require.NoError(t, err)
		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("http://127.0.0.1:%s/api/v1/auth/ssh-keys", backendPort.Port()),
			bytes.NewBuffer(body),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := user.Client.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	}
}
