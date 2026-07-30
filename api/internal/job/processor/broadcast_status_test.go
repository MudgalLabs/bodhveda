package processor

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
)

// testPool opens the integration-test pool, skipping when TEST_DB_URL is unset.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// testProject creates a scratch project, cleaned up with the test.
func testProject(t *testing.T, pool *pgxpool.Pool, name string) int {
	t.Helper()
	ctx := context.Background()

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, $2, now(), now()) RETURNING id
	`, userID, name).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	return projectID
}

// TestBroadcastDeliveryWritesDeliveredNotifications pins the fix for the bug
// where every broadcast notification sat permanently in `enqueued`.
//
// entity.NewNotification defaults to `enqueued`, which is right for a direct
// send (notification:delivery still has to resolve preferences and billing) but
// wrong for a broadcast, where prepare_batches already did that and nothing
// further will ever touch the row. Leaving it non-terminal made the console
// delivery tree report every broadcast as 100% pending and made
// internal/monitor's stuck_sends check alert on all of them, forever.
func TestBroadcastDeliveryWritesDeliveredNotifications(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-status-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	batch, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, 2))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
		ProjectID:       projectID,
		BroadcastID:     broadcast.ID,
		BatchID:         batch.ID,
		RecipientExtIDs: []string{"r1", "r2"},
		Payload:         []byte(`{"t":"hi"}`),
		Channel:         "product",
		Topic:           "updates",
		Event:           "released",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	p := NewBroadcastDeliveryProcessor(pool, notificationRepo, broadcastRepo, batchRepo)
	if err := p.ProcessTask(ctx, asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)); err != nil {
		t.Fatalf("process task: %v", err)
	}

	rollup, err := notificationRepo.StatusRollupForBroadcast(ctx, broadcast.ID)
	if err != nil {
		t.Fatalf("rollup: %v", err)
	}

	if got := rollup[enum.NotificationStatusDelivered]; got != 2 {
		t.Errorf("delivered = %d, want 2 (rollup: %v)", got, rollup)
	}
	if got := rollup[enum.NotificationStatusEnqueued]; got != 0 {
		t.Errorf("enqueued = %d, want 0 — a broadcast notification has no second step to wait for", got)
	}

	// The property both the tree and the monitor depend on: nothing is left in a
	// non-terminal state once the batch is written.
	for status, count := range rollup {
		if !status.Terminal() && count > 0 {
			t.Errorf("status %q is non-terminal with %d rows; broadcast delivery must leave nothing pending", status, count)
		}
	}

	// completed_at must be stamped, like the direct path does when it resolves.
	var missing int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notification WHERE broadcast_id = $1 AND completed_at IS NULL
	`, broadcast.ID).Scan(&missing); err != nil {
		t.Fatalf("count completed_at: %v", err)
	}
	if missing != 0 {
		t.Errorf("%d delivered notifications have no completed_at", missing)
	}
}

// TestPrepareBatchesCompletesEmptyBroadcast pins the fix for a broadcast that
// matches nobody.
//
// It used to do two wrong things at once: `usage_log` has CHECK (amount > 0), so
// consuming 0 units raised SQLSTATE 23514 and Asynq retried the task until it was
// ARCHIVED; and with no batches, nothing was left to mark the broadcast
// `completed`, so it sat in `enqueued` forever.
//
// ⚠️ The asynq client and billing service are deliberately passed as nil. On this
// path neither may be reached — no usage is consumed and no batch is enqueued —
// so a nil panic here is the assertion.
func TestPrepareBatchesCompletesEmptyBroadcast(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-empty-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	preferenceRepo := pg.NewPreferenceRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	// A recipient exists, but nothing makes the target eligible for in_app — no
	// catalog row at all. This is the "broadcast silently reaches nobody" case.
	if _, err := pool.Exec(ctx, `
		INSERT INTO recipient (project_id, external_id, created_at, updated_at)
		VALUES ($1, 'nobody', now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{}`), "promo", "none", "sent"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	payload, err := json.Marshal(dto.PrepareBroadcastBatchesPayload{
		UserID:    1,
		Broadcast: broadcast,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	p := NewPrepareBroadcastBatchesProcessor(pool, nil, preferenceRepo, broadcastRepo, batchRepo, nil)
	if err := p.ProcessTask(ctx, asynq.NewTask(task.TaskTypePrepareBroadcastBatches, payload)); err != nil {
		t.Fatalf("an empty audience is a normal outcome, not an error: %v", err)
	}

	reloaded, err := broadcastRepo.GetByID(ctx, broadcast.ID)
	if err != nil {
		t.Fatalf("reload broadcast: %v", err)
	}

	// Without the fix this stays `enqueued` forever: no batch exists to complete it.
	if reloaded.Status != enum.BroadcastStatusCompleted {
		t.Errorf("status = %q, want completed — nothing else will ever move it", reloaded.Status)
	}
	if reloaded.CompletedAt == nil {
		t.Error("completed_at should be stamped")
	}

	// The audience must still be recorded — it is the whole diagnosis of why the
	// broadcast reached nobody.
	if reloaded.Audience == nil {
		t.Fatal("audience must be recorded even when nobody is eligible")
	}
	if reloaded.Audience.Eligible != 0 {
		t.Errorf("eligible = %d, want 0", reloaded.Audience.Eligible)
	}
	if reloaded.Audience.ExcludedNotCataloged != 1 {
		t.Errorf("excluded_not_cataloged = %d, want 1 — this is the operator's answer to 'why did nobody get it?'",
			reloaded.Audience.ExcludedNotCataloged)
	}

	var batches int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM broadcast_batch WHERE broadcast_id = $1`, broadcast.ID).Scan(&batches); err != nil {
		t.Fatalf("count batches: %v", err)
	}
	if batches != 0 {
		t.Errorf("no batches should be created for an empty audience, got %d", batches)
	}
}
