package processor

import (
	"context"
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// emailTestService builds a NotificationService over real repositories.
//
// billing, recipient service and the asynq client are nil: the broadcast email
// paths under test must reach none of them (the service only BUILDS the send
// tasks — the processor enqueues them after commit), so a nil panic is an
// assertion that the layering held.
func emailTestService(pool *pgxpool.Pool) *service.NotificationService {
	return service.NewNotificationService(
		pg.NewNotificationRepo(pool), pg.NewRecipientRepo(pool), pg.NewPreferenceRepo(pool),
		pg.NewBroadcastRepo(pool), pg.NewBroadcastBatchRepo(pool),
		pg.NewNotificationDeliveryRepo(pool), pg.NewRecipientContactRepo(pool),
		pg.NewProjectEmailSettingsRepo(pool),
		nil, nil, nil,
	)
}

// seedEmailProject wires a project that can send email: settings row, a catalog
// entry enabling the target on BOTH in_app and email, recipients, and a primary
// email contact for each recipient named in withContact.
func seedEmailProject(t *testing.T, pool *pgxpool.Pool, projectID int, cap int, recipients []string, withContact []string) {
	t.Helper()
	ctx := context.Background()

	// secret/nonce are NOT NULL but never decrypted on this path — the fan-out
	// only reads the provider name and the cap.
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_email_settings
			(project_id, provider, secret, nonce, from_name, from_address,
			 max_broadcast_recipients_for_email, created_at, updated_at)
		VALUES ($1, 'resend', '\x00', '\x00', 'Test', 'test@example.com', $2, now(), now())
	`, projectID, cap); err != nil {
		t.Fatalf("insert email settings: %v", err)
	}

	for _, medium := range []string{"in_app", "email"} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, name, medium, enabled, created_at, updated_at)
			VALUES ($1, NULL, 'product', 'updates', 'released', 'Updates', $2, true, now(), now())
		`, projectID, medium); err != nil {
			t.Fatalf("insert %s catalog preference: %v", medium, err)
		}
	}

	for _, extID := range recipients {
		if _, err := pool.Exec(ctx, `
			INSERT INTO recipient (project_id, external_id, created_at, updated_at)
			VALUES ($1, $2, now(), now())
		`, projectID, extID); err != nil {
			t.Fatalf("insert recipient %s: %v", extID, err)
		}
	}

	for _, extID := range withContact {
		if _, err := pool.Exec(ctx, `
			INSERT INTO recipient_contact
				(project_id, recipient_external_id, medium, address, is_primary, created_at, updated_at)
			VALUES ($1, $2, 'email', $3, true, now(), now())
		`, projectID, extID, extID+"@example.com"); err != nil {
			t.Fatalf("insert contact for %s: %v", extID, err)
		}
	}
}

// TestBroadcastEmailFansOutToEligibleRecipients is the core of broadcast email:
// the right people get a send task, and everyone the broadcast intended to mail
// gets a delivery row recording what happened — including the ones that could not
// be mailed.
func TestBroadcastEmailFansOutToEligibleRecipients(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-email-fanout-test")

	// r3 has no email contact: eligible by preference, unmailable in practice.
	seedEmailProject(t, pool, projectID, 100, []string{"r1", "r2", "r3"}, []string{"r1", "r2"})

	broadcastRepo := pg.NewBroadcastRepo(pool)
	svc := emailTestService(pool)

	broadcast, err := broadcastRepo.Create(ctx,
		entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released").
			WithEmail("Release 2.0", "<p>hi</p>", "hi"),
	)
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}
	if broadcast.Email == nil {
		t.Fatal("email content did not round-trip through Create")
	}

	notifications := []*entity.Notification{}
	for _, extID := range []string{"r1", "r2", "r3"} {
		notifications = append(notifications,
			entity.NewNotification(projectID, extID, []byte(`{"t":"hi"}`), &broadcast.ID, "product", "updates", "released"))
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := pg.NewNotificationRepo(pool).BatchCreateTx(ctx, tx, notifications); err != nil {
		t.Fatalf("insert notifications: %v", err)
	}

	target := dto.Target{Channel: "product", Topic: "updates", Event: "released"}
	eligible, err := svc.ResolveBroadcastEmailAudience(ctx, tx, broadcast, target, []string{"r1", "r2", "r3"})
	if err != nil {
		t.Fatalf("resolve email audience: %v", err)
	}
	if len(eligible) != 3 {
		t.Fatalf("email-eligible = %d, want 3 (all three are cataloged for email)", len(eligible))
	}

	tasks, err := svc.FanOutBroadcastEmail(ctx, tx, broadcast, notifications)
	if err != nil {
		t.Fatalf("fan out: %v", err)
	}

	// Only the two with an address get a send task. r3 must not.
	if len(tasks) != 2 {
		t.Fatalf("send tasks = %d, want 2 — only recipients with an address can be mailed", len(tasks))
	}
	got := map[string]dto.EmailDeliveryTaskPayload{}
	for _, task := range tasks {
		got[task.To] = task
	}
	for _, addr := range []string{"r1@example.com", "r2@example.com"} {
		task, ok := got[addr]
		if !ok {
			t.Fatalf("no send task for %s; tasks went to %v", addr, got)
		}
		if task.Subject != "Release 2.0" {
			t.Errorf("subject = %q, want the broadcast's own subject", task.Subject)
		}
		if task.DeliveryID == 0 {
			t.Errorf("task for %s has no delivery id to record its outcome against", addr)
		}
	}

	// r3 still gets a row — an unmailable recipient is a visible outcome, not a
	// silent omission.
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM notification_delivery
		WHERE project_id = $1 AND recipient_external_id = 'r3' AND medium = 'email'
	`, projectID).Scan(&status); err != nil {
		t.Fatalf("r3 has no delivery row; an unmailable recipient must still be recorded: %v", err)
	}
	if status != string(enum.DeliverySkippedNoContact) {
		t.Errorf("r3 status = %q, want %q", status, enum.DeliverySkippedNoContact)
	}

	// Every delivery row must point at its own recipient's notification.
	rows, err := tx.Query(ctx, `
		SELECT d.recipient_external_id, n.recipient_external_id
		FROM notification_delivery d JOIN notification n ON n.id = d.notification_id
		WHERE d.project_id = $1 AND d.medium = 'email'
	`, projectID)
	if err != nil {
		t.Fatalf("join deliveries: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var deliveryFor, notificationFor string
		if err := rows.Scan(&deliveryFor, &notificationFor); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if deliveryFor != notificationFor {
			t.Errorf("delivery for %q hangs off %q's notification — ids were misattributed", deliveryFor, notificationFor)
		}
	}
}

// TestBroadcastEmailCapBlocksRatherThanTruncates pins the safety rail.
//
// ⚠️ The cap must BLOCK, not truncate. Mailing the first N of an over-cap
// audience picks an arbitrary subset by query order and looks like success, which
// is harder to notice than nothing being sent.
func TestBroadcastEmailCapBlocksRatherThanTruncates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-email-cap-test")

	recipients := []string{"r1", "r2", "r3"}
	seedEmailProject(t, pool, projectID, 2, recipients, recipients) // cap 2, audience 3

	broadcastRepo := pg.NewBroadcastRepo(pool)
	svc := emailTestService(pool)

	broadcast, err := broadcastRepo.Create(ctx,
		entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released").
			WithEmail("Too many", "<p>x</p>", "x"),
	)
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	target := dto.Target{Channel: "product", Topic: "updates", Event: "released"}
	eligible, err := svc.ResolveBroadcastEmailAudience(ctx, tx, broadcast, target, recipients)
	if err != nil {
		t.Fatalf("resolve email audience: %v", err)
	}
	if len(eligible) != 0 {
		t.Fatalf("over-cap audience returned %d recipients to mail; the cap must block entirely", len(eligible))
	}

	var eligibleCount int
	var reason *string
	if err := tx.QueryRow(ctx, `
		SELECT email_eligible_recipients, email_blocked_reason FROM broadcast WHERE id = $1
	`, broadcast.ID).Scan(&eligibleCount, &reason); err != nil {
		t.Fatalf("read email outcome: %v", err)
	}
	// The count is still recorded: "3 people would have been mailed" is the whole
	// diagnosis of why nothing went out.
	if eligibleCount != 3 {
		t.Errorf("email_eligible_recipients = %d, want 3 — the blocked audience must still be recorded", eligibleCount)
	}
	if reason == nil || *reason != entity.EmailBlockedRecipientCapExceeded {
		t.Fatalf("email_blocked_reason = %v, want %q", reason, entity.EmailBlockedRecipientCapExceeded)
	}

	// And nothing may be mailed on a blocked broadcast, even if fan-out is called.
	blocked, err := broadcastRepo.GetByID(ctx, broadcast.ID)
	if err != nil {
		t.Fatalf("reload broadcast: %v", err)
	}
	tasks, err := svc.FanOutBroadcastEmail(ctx, tx, blocked, nil)
	if err != nil {
		t.Fatalf("fan out on blocked broadcast: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("blocked broadcast produced %d send tasks", len(tasks))
	}
}

// TestFailedBatchKeepsBroadcastIncomplete pins that a broadcast does not claim to
// have completed while one of its batches delivered nothing.
//
// PendingCount used to count only `enqueued` batches, so a `failed` one was
// invisible: remaining hit 0 and the broadcast was marked `completed`. With in-app
// only that was a cosmetic lie. With email it says "completed" when people were
// never mailed.
func TestFailedBatchKeepsBroadcastIncomplete(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := testProject(t, pool, "bcast-failed-batch-test")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{}`), "product", "updates", "released"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	ok, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, []string{"r1"}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	bad, err := batchRepo.Create(ctx, entity.NewBroadcastBatch(broadcast.ID, []string{"r2"}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	if err := batchRepo.Update(ctx, ok.ID, entity.NewBroadcastBatchUpdatePayload(enum.BroadcastBatchStatusSuccess, 1, 1)); err != nil {
		t.Fatalf("mark success: %v", err)
	}
	if err := batchRepo.Update(ctx, bad.ID, entity.NewBroadcastBatchUpdatePayload(enum.BroadcastBatchStatusFailed, 3, 1)); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	remaining, err := batchRepo.PendingCount(ctx, broadcast.ID)
	if err != nil {
		t.Fatalf("pending count: %v", err)
	}
	if remaining == 0 {
		t.Fatal("a failed batch counted as done; the broadcast would be marked completed having delivered nothing for it")
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1 (the failed batch)", remaining)
	}
}
