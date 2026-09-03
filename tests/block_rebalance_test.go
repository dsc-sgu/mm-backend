package tests

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/dsc-sgu/mm-backend/internal/blocks"
)

// TestBlockPositionRebalancing verifies that once a block's lexorank
// position grows past the configured threshold, the background rebalance
// worker eventually shrinks every position back down without disturbing
// block order.
func TestBlockPositionRebalancing(t *testing.T) {
	ctx := context.Background()

	net, err := network.New(ctx)
	if err != nil {
		t.Fatal("create network:", err)
	}
	defer func() {
		if err := net.Remove(ctx); err != nil {
			t.Logf("Failed to remove network: %v", err)
		}
	}()

	pgContainer, pgPort, err := initPostgres(ctx, net)
	if err != nil {
		t.Fatal("start postgres:", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate postgres container: %v", err)
		}
	}()
	dsn := fmt.Sprintf(
		"host=127.0.0.1 port=%s user=postgres password=postgres dbname=postgres sslmode=disable",
		pgPort.Port(),
	)
	pgDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("connect to postgres:", err)
	}
	if err := pgDB.PingContext(ctx); err != nil {
		t.Fatal("ping postgres:", err)
	}
	defer func() {
		if err := pgDB.Close(); err != nil {
			t.Logf("Failed to close postgres connection: %v", err)
		}
	}()

	redisContainer, redisPort, err := initRedis(ctx, net)
	if err != nil {
		t.Fatal("start redis:", err)
	}
	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate redis container: %v", err)
		}
	}()
	redisAddr := fmt.Sprintf("127.0.0.1:%s", redisPort.Port())
	redisDB := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := redisDB.Ping(ctx).Err(); err != nil {
		t.Fatal("ping redis:", err)
	}
	defer func() {
		if err := redisDB.Close(); err != nil {
			t.Logf("Failed to close redis connection: %v", err)
		}
	}()

	// A tiny threshold means a handful of insertions is enough to trigger a
	// rebalance, instead of needing dozens under the production default.
	backendEnv := map[string]string{"LEXO_RANK_THRESHOLD": "2"}
	backendContainer, backendPort, _, err := initBackendWithEnv(
		ctx,
		net,
		backendEnv,
	)
	if err != nil {
		t.Fatal("start backend:", err)
	}
	defer func() {
		if err := backendContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate backend container: %v", err)
		}
	}()

	testUser := CreateAndLoginUser(
		t,
		backendPort,
		"Rebalance",
		"Tester",
		"rebalance-tester",
		"rebalance@test.com",
		"password",
	)
	disciplineID := CreateTestDiscipline(
		t,
		backendPort,
		&testUser,
		"Rebalance Discipline",
	)
	courseID := CreateTestCourse(
		t,
		backendPort,
		&testUser,
		disciplineID,
		"Rebalance Course",
		"Rebalance Course",
	)
	draftSnapshotID := LockCourse(
		t,
		backendPort,
		&testUser,
		courseID,
	).DraftSnapshotID

	// The anchor stays the first block throughout; every subsequent block
	// is inserted immediately after it. Since the anchor's own position
	// never moves, each new insertion squeezes into an ever-shrinking gap
	// between the anchor and whatever currently follows it - the worst
	// case for fractional indexing, which reliably grows the position
	// string past any threshold within a handful of insertions.
	anchorID := CreateTestBlockAfter(
		t,
		backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
		nil,
	)
	neighborID := CreateTestBlockAfter(
		t,
		backendPort,
		&testUser,
		courseID,
		draftSnapshotID,
		&anchorID,
	)

	const insertions = 30
	insertedOrder := make([]uuid.UUID, insertions)
	for i := range insertions {
		insertedOrder[i] = CreateTestBlockAfter(
			t,
			backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
			&anchorID,
		)
	}

	// Expected order: anchor first, then every inserted block in reverse
	// insertion order (each new one lands right after the anchor, pushing
	// the rest down), then the original neighbor last.
	expectedOrder := make([]uuid.UUID, 0, insertions+2)
	expectedOrder = append(expectedOrder, anchorID)
	for i := insertions - 1; i >= 0; i-- {
		expectedOrder = append(expectedOrder, insertedOrder[i])
	}
	expectedOrder = append(expectedOrder, neighborID)

	// The rebalance worker runs asynchronously in the background, so poll
	// until every position has shrunk back down. IndexToPosition never
	// produces more than 4 characters, so this is a precise, not just
	// approximate, signal that a rebalance has completed.
	var finalBlocks []*blocks.Block
	deadline := time.Now().Add(15 * time.Second)
	for {
		finalBlocks = GetSnapshotBlocks(
			t,
			backendPort,
			&testUser,
			courseID,
			draftSnapshotID,
		)

		longest := 0
		for _, b := range finalBlocks {
			if len(b.Position) > longest {
				longest = len(b.Position)
			}
		}
		if longest <= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"rebalance did not bring positions back down in time, longest position length = %d",
				longest,
			)
		}
		time.Sleep(200 * time.Millisecond)
	}

	require.Len(t, finalBlocks, insertions+2)

	finalOrder := make([]uuid.UUID, len(finalBlocks))
	for i, b := range finalBlocks {
		finalOrder[i] = b.ID
	}
	require.Equal(
		t,
		expectedOrder,
		finalOrder,
		"block order must be unchanged after rebalancing",
	)
}
