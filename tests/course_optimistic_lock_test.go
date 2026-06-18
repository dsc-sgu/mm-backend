package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

func TestOptimisticLockingConflictScenario(t *testing.T) {
	ctx := context.Background()

	// Setup network
	net, err := network.New(ctx)
	if err != nil {
		t.Fatal("create network:", err)
	}
	defer net.Remove(ctx)

	// Setup Postgres
	pgContainer, pgPort, err := initPostgres(ctx, net)
	if err != nil {
		t.Fatal("start postgres:", err)
	}
	defer pgContainer.Terminate(ctx)
	dsn := fmt.Sprintf(
		"host=127.0.0.1 port=%s user=postgres password=postgres dbname=postgres sslmode=disable",
		pgPort.Port(),
	)
	pgDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("connect to postgres:", err)
	}
	if err := pgDB.PingContext(ctx); err != nil {
		log.Fatal("ping postgres:", err)
	}
	defer pgDB.Close()

	// Setup Redis
	redisContainer, redisPort, err := initRedis(ctx, net)
	if err != nil {
		t.Fatal("start redis:", err)
	}
	defer redisContainer.Terminate(ctx)
	redisAddr := fmt.Sprintf("127.0.0.1:%s", redisPort.Port())
	redisDB := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := redisDB.Ping(ctx).Err(); err != nil {
		log.Fatal("ping redis:", err)
	}
	defer redisDB.Close()

	// Setup Backend with short lock TTL
	backendEnv := map[string]string{"COURSE_LOCK_TTL_SECONDS": "2"}
	backendContainer, backendPort, err := initBackendWithEnv(ctx, net, backendEnv)
	if err != nil {
		t.Fatal("start backend:", err)
	}
	defer backendContainer.Terminate(ctx)

	// --- Test Scenario ---

	// 1. Create two teachers and a course
	userA := CreateAndLoginUser(t, backendPort, "User", "A", "user-a", "a@test.com", "password")
	userB := CreateAndLoginUser(t, backendPort, "User", "B", "user-b", "b@test.com", "password")
	disciplineID := CreateTestDiscipline(t, backendPort, &userA, "Optimistic Lock Discipline")
	courseID := CreateTestCourse(t, backendPort, &userA, disciplineID, "Optimistic Lock Course")

	// 2. Add User B as a teacher to the course
	{
		// This is a simplified version of the invite logic from the other test file.
		// In a real project, this would be a shared helper.
		// For this isolated test, it is duplicated.
		inviteRes, err := createInvite(t, backendPort, &userA, courseID, courses.TeacherRole)
		require.NoError(t, err)
		err = joinCourse(t, backendPort, &userB, inviteRes.ID)
		require.NoError(t, err)
	}

	// 3. User A locks the course and gets a draft.
	lockResultA := LockCourse(t, backendPort, &userA, courseID)
	userADraftID := lockResultA.DraftSnapshotID
	require.Equal(t, "new", lockResultA.InitType)

	// 4. Wait for User A's lock to expire.
	t.Log("Waiting for User A's lock to expire (3 seconds)...")
	time.Sleep(3 * time.Second)

	// 5. User B now acquires the lock, which succeeds because A's lock is expired.
	// User B gets a *new* draft.
	lockResultB := LockCourse(t, backendPort, &userB, courseID)
	userBDraftID := lockResultB.DraftSnapshotID
	require.Equal(t, "new", lockResultB.InitType)
	require.NotEqual(t, userADraftID, userBDraftID)

	// 6. User B makes a change and publishes successfully.
	CreateTestBlock(t, backendPort, &userB, userBDraftID)
	publishStatusCode := PublishDraft(t, backendPort, &userB, courseID, userBDraftID)
	require.Equal(t, http.StatusNoContent, publishStatusCode)
	t.Log("User B published successfully, incrementing course version.")

	// 7. User A comes back and tries to re-acquire the lock.
	// The system should find User A's old draft and report a stale conflict.
	staleLockResultA := LockCourse(t, backendPort, &userA, courseID)
	require.Equal(t, "stale_conflict", staleLockResultA.InitType)
	require.Equal(t, userADraftID, staleLockResultA.DraftSnapshotID)
	t.Log("User A re-acquired lock and was correctly notified of stale draft.")

	// 8. User A ignores the warning and tries to publish their stale draft anyway.
	// This must fail with a 409 Conflict because the course version has changed.
	t.Log("User A is attempting to publish stale draft...")
	finalPublishStatusCode := PublishDraft(t, backendPort, &userA, courseID, userADraftID)
	require.Equal(t, http.StatusConflict, finalPublishStatusCode)
	t.Log("User A's publish failed with 409 Conflict as expected.")
}

// Minimal helpers for this isolated test to avoid circular dependencies if moved.
func createInvite(t *testing.T, port *nat.Port, user *TestUser, courseID uuid.UUID, role courses.CourseMemberRole) (courses.Invite, error) {
	inviteURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites", port.Port())
	expiresAt := time.Now().Add(24 * time.Hour)
	inviteBody, _ := json.Marshal(courses.CreateInvite{CourseID: courseID, ProvidedRole: role, ExpiresAt: &expiresAt})
	req, err := http.NewRequest(http.MethodPost, inviteURL, bytes.NewBuffer(inviteBody))
	if err != nil {
		return courses.Invite{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := user.Client.Do(req)
	if err != nil {
		return courses.Invite{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return courses.Invite{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var invite courses.Invite
	err = json.NewDecoder(resp.Body).Decode(&invite)
	return invite, err
}
func joinCourse(t *testing.T, port *nat.Port, user *TestUser, inviteID uuid.UUID) error {
	joinURL := fmt.Sprintf("http://127.0.0.1:%s/api/v1/course-invites/%s", port.Port(), inviteID)
	req, err := http.NewRequest(http.MethodPost, joinURL, nil)
	if err != nil {
		return err
	}
	resp, err := user.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

