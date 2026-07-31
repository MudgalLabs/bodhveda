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

	// ⚠️ Delivery re-derives in-app eligibility per batch (a batch can contain
	// email-only recipients since the fan-out audience became a union), so the
	// catalog entry that makes these two in-app eligible is now a real
	// precondition of this test rather than an unstated assumption. Without it
	// every row is correctly `muted` and this test would be asserting the wrong
	// thing.
	if _, err := pool.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, name, medium, enabled, created_at, updated_at)
		VALUES ($1, NULL, 'product', 'updates', 'released', 'Updates', 'in_app', true, now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert in_app catalog preference: %v", err)
	}
	for _, extID := range []string{"r1", "r2"} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO recipient (project_id, external_id, created_at, updated_at) VALUES ($1, $2, now(), now())
		`, projectID, extID); err != nil {
			t.Fatalf("insert recipient %s: %v", extID, err)
		}
	}

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	batch, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, []string{"r1", "r2"}))
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

	p := NewBroadcastDeliveryProcessor(pool, notificationRepo, broadcastRepo, batchRepo, pg.NewPreferenceRepo(pool), nil, nil)
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

	p := NewPrepareBroadcastBatchesProcessor(pool, nil, preferenceRepo, broadcastRepo, batchRepo, nil, nil)
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

// TestBroadcastDeliveryIsIdempotentOnRetry pins the fix for the duplicate-fan-out
// bug.
//
// Asynq is at-least-once: it acks only after ProcessTask returns, so a crash or
// an expired lease between the commit and the ack re-delivers the batch. The
// fan-out used to commit in its own transaction and write the batch status in a
// second one, so a redelivery re-ran the insert and wrote every notification
// again. Today that is duplicate rows in real inboxes; with broadcast email it
// would be duplicate mail to real people.
//
// Running the identical task twice is exactly what a redelivery looks like to
// this processor, so that is what the test does.
func TestBroadcastDeliveryIsIdempotentOnRetry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-idempotency-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	batch, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, []string{"r1", "r2", "r3"}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
		ProjectID:       projectID,
		BroadcastID:     broadcast.ID,
		BatchID:         batch.ID,
		RecipientExtIDs: []string{"r1", "r2", "r3"},
		Payload:         []byte(`{"t":"hi"}`),
		Channel:         "product",
		Topic:           "updates",
		Event:           "released",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	p := NewBroadcastDeliveryProcessor(pool, notificationRepo, broadcastRepo, batchRepo, pg.NewPreferenceRepo(pool), nil, nil)
	t2 := asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)

	if err := p.ProcessTask(ctx, t2); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// The redelivery. It must succeed — a retry of already-committed work is not
	// an error, it is a no-op — and must not write a second copy of anything.
	if err := p.ProcessTask(ctx, t2); err != nil {
		t.Fatalf("second run (the redelivery) must be a successful no-op: %v", err)
	}

	var total int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notification WHERE broadcast_id = $1
	`, broadcast.ID).Scan(&total); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if total != 3 {
		t.Fatalf("notification count = %d, want 3 — the redelivery duplicated the fan-out", total)
	}

	// Per recipient, not just in aggregate: three rows could also be one recipient
	// written three times.
	rows, err := pool.Query(ctx, `
		SELECT recipient_external_id, COUNT(*)
		FROM notification WHERE broadcast_id = $1
		GROUP BY recipient_external_id HAVING COUNT(*) > 1
	`, broadcast.ID)
	if err != nil {
		t.Fatalf("query duplicates: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var extID string
		var n int
		if err := rows.Scan(&extID, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Errorf("recipient %q got %d notifications from one broadcast, want 1", extID, n)
	}

	// The broadcast must still be completed — the retry skips the insert but must
	// NOT skip the completion check, or a crash between commit and ack would
	// strand the broadcast in `enqueued` forever.
	reloaded, err := broadcastRepo.GetByID(ctx, broadcast.ID)
	if err != nil {
		t.Fatalf("reload broadcast: %v", err)
	}
	if reloaded.Status != enum.BroadcastStatusCompleted {
		t.Errorf("broadcast status = %q, want %q after the retry", reloaded.Status, enum.BroadcastStatusCompleted)
	}
}

// TestBroadcastDeliveryReturnsErrorSoAsynqRetries pins the swallowed-error half of
// the same bug.
//
// The delivery error used to be overwritten by the batch-status update's own
// (nil) error, and ProcessTask returned nil — so a batch that inserted NOTHING
// was acked as a success and never retried. The failure was written to the
// database and then dropped on the floor, which is precisely the silent-work-loss
// shape of the 2026-07-27 incident.
func TestBroadcastDeliveryReturnsErrorSoAsynqRetries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-retry-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	// A batch id that does not exist stands in for any failure inside the
	// transaction: the work cannot be recorded, so the task must report failure
	// rather than claim success.
	payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
		ProjectID:       projectID,
		BroadcastID:     broadcast.ID,
		BatchID:         -1,
		RecipientExtIDs: []string{"r1"},
		Payload:         []byte(`{"t":"hi"}`),
		Channel:         "product",
		Topic:           "updates",
		Event:           "released",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	p := NewBroadcastDeliveryProcessor(pool, notificationRepo, broadcastRepo, batchRepo, pg.NewPreferenceRepo(pool), nil, nil)
	if err := p.ProcessTask(ctx, asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)); err == nil {
		t.Fatal("ProcessTask returned nil for a batch that could not be delivered; Asynq would ack it and never retry")
	}

	// And nothing may be left behind by the rolled-back attempt.
	var written int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notification WHERE broadcast_id = $1
	`, broadcast.ID).Scan(&written); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if written != 0 {
		t.Errorf("%d notifications survived a failed batch; the insert must roll back with it", written)
	}
}

// TestPrepareBatchesDoesNotRePrepareOnRetry pins the fix for the
// prepare_batches idempotency bug.
//
// Everything in preparation used to run unguarded on every delivery of the task,
// so an Asynq retry re-listed the audience, consumed the quota A SECOND TIME, and
// created a whole new set of batches. Those duplicates carry fresh batch ids, so
// BroadcastDeliveryProcessor's per-batch guard does not catch them — every one
// fans out and every recipient receives the broadcast again.
//
// ⚠️ The asynq client and the billing service are deliberately nil. A retry must
// touch neither: the quota was consumed by the first attempt, and a batch already
// marked `success` has nothing left to enqueue. Reaching either is a nil panic,
// which is the assertion.
func TestPrepareBatchesDoesNotRePrepareOnRetry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-prepare-idem-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	preferenceRepo := pg.NewPreferenceRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	// ⚠️ The audience must be NON-EMPTY for this test to mean anything. With no
	// eligible recipient the unguarded path takes the empty-audience early return,
	// which creates no batch and never reaches billing — so the test would pass
	// even with the guard removed. A recipient plus an enabled project-level
	// catalog entry is what makes re-preparation actually do damage.
	if _, err := pool.Exec(ctx, `
		INSERT INTO recipient (project_id, external_id, created_at, updated_at)
		VALUES ($1, 'r1', now(), now()), ($1, 'r2', now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert recipients: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, name, medium, enabled, created_at, updated_at)
		VALUES ($1, NULL, 'product', 'updates', 'released', 'Updates', 'in_app', true, now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert catalog preference: %v", err)
	}

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	// Stand in for a first attempt that completed preparation: one batch exists
	// and has already been delivered.
	batch, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, []string{"r1", "r2"}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if err := batchRepo.Update(ctx, batch.ID, entity.NewBroadcastBatchUpdatePayload(
		enum.BroadcastBatchStatusSuccess, 1, 5,
	)); err != nil {
		t.Fatalf("mark batch success: %v", err)
	}

	payload, err := json.Marshal(dto.PrepareBroadcastBatchesPayload{
		UserID:    1,
		Broadcast: broadcast,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	p := NewPrepareBroadcastBatchesProcessor(pool, nil, preferenceRepo, broadcastRepo, batchRepo, nil, nil)
	if err := p.ProcessTask(ctx, asynq.NewTask(task.TaskTypePrepareBroadcastBatches, payload)); err != nil {
		t.Fatalf("a redelivery of an already-prepared broadcast must be a no-op: %v", err)
	}

	var batches int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM broadcast_batch WHERE broadcast_id = $1`, broadcast.ID).Scan(&batches); err != nil {
		t.Fatalf("count batches: %v", err)
	}
	if batches != 1 {
		t.Errorf("batch count = %d, want 1 — the retry re-prepared the broadcast", batches)
	}
}

// TestBroadcastBatchPersistsItsRecipients pins the column that makes preparation
// resumable, and the rule that a batch which cannot be resumed is skipped rather
// than treated as empty.
//
// Before this, the batch→recipient mapping existed only in the Asynq payload, so
// a batch whose task was never enqueued could not be recovered by anything.
func TestBroadcastBatchPersistsItsRecipients(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-batch-recipients-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	want := []string{"r1", "r2", "r3"}

	created, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, want))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if len(created.RecipientExtIDs) != len(want) {
		t.Fatalf("recipient ids round-tripped as %v, want %v", created.RecipientExtIDs, want)
	}
	if created.Recipients != len(want) {
		t.Errorf("recipients count = %d, want %d — it must stay consistent with the id list", created.Recipients, len(want))
	}

	// A legacy batch: no recipient ids, because the column did not exist when it
	// was written. It must NOT come back as resumable — delivering to an empty set
	// would mark it successful having reached nobody.
	var legacyID int
	if err := pool.QueryRow(ctx, `
		INSERT INTO broadcast_batch (broadcast_id, recipients, status, attempt, duration, created_at, updated_at)
		VALUES ($1, 5, 'enqueued', 0, 0, now(), now()) RETURNING id
	`, broadcast.ID).Scan(&legacyID); err != nil {
		t.Fatalf("insert legacy batch: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	resumable, err := batchRepo.ResumableForBroadcastTx(ctx, tx, broadcast.ID)
	if err != nil {
		t.Fatalf("list resumable: %v", err)
	}
	if len(resumable) != 1 {
		t.Fatalf("resumable = %d batches, want 1 (the legacy NULL-recipient batch must be skipped)", len(resumable))
	}
	if resumable[0].ID != created.ID {
		t.Errorf("resumable batch id = %d, want %d", resumable[0].ID, created.ID)
	}
	if len(resumable[0].RecipientExtIDs) != len(want) {
		t.Errorf("resumable batch carries %v, want %v", resumable[0].RecipientExtIDs, want)
	}
}

// TestBroadcastDeliveryBackfillsNotificationIDs pins the switch from COPY to
// INSERT ... RETURNING.
//
// COPY is faster but returns nothing, and a notification_delivery row hangs off
// notification_id — so without ids there is no way to record a per-recipient
// email outcome. That, not idempotency, is why the insert stopped being a COPY.
//
// ⚠️ Also pins that ids are matched back by recipient_external_id rather than by
// RETURNING order. Postgres does not guarantee RETURNING comes back in input
// order, and trusting it would misattribute delivery rows — mailing the right
// content against the wrong person's row — the first time the planner chose
// differently.
func TestBroadcastDeliveryBackfillsNotificationIDs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-ids-test")

	notificationRepo := pg.NewNotificationRepo(pool)
	broadcastRepo := pg.NewBroadcastRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	extIDs := []string{"z_last", "a_first", "m_middle"}
	notifications := make([]*entity.Notification, 0, len(extIDs))
	for _, extID := range extIDs {
		n := entity.NewNotification(projectID, extID, []byte(`{"t":"hi"}`), &broadcast.ID, "product", "updates", "released")
		notifications = append(notifications, n)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := notificationRepo.BatchCreateTx(ctx, tx, notifications); err != nil {
		t.Fatalf("batch create: %v", err)
	}

	for _, n := range notifications {
		if n.ID == 0 {
			t.Fatalf("notification for %q has no id; delivery rows cannot hang off it", n.RecipientExtID)
		}
	}

	// Each id must belong to the recipient it was written against. Deliberately
	// checked against the database rather than the in-memory slice, so an
	// order-based mapping bug cannot pass by being wrong consistently.
	for _, n := range notifications {
		var got string
		if err := tx.QueryRow(ctx, `SELECT recipient_external_id FROM notification WHERE id = $1`, n.ID).Scan(&got); err != nil {
			t.Fatalf("look up notification %d: %v", n.ID, err)
		}
		if got != n.RecipientExtID {
			t.Errorf("id %d was mapped to %q but belongs to %q — ids are being matched by RETURNING order", n.ID, n.RecipientExtID, got)
		}
	}
}
