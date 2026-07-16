package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/tantra/query"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestNotificationDeliveryDetail covers Phase 9.1's projection widening against a
// real Postgres: the notifications LIST must carry every bounded delivery column
// (notably failure_reason, the only thing separating the two causes of `muted`),
// and the per-notification deliveries endpoint must return the unbounded
// provider_response history normalized into timeline events.
//
// Skipped unless TEST_DB_URL is set. Uses scratch project id 1 (the convention of
// the other pg/service integration tests) and cleans up after itself.
func TestNotificationDeliveryDetail(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const projectID = 1
	const recipientExtID = "dd-test-recipient"
	const providerMessageID = "dd_test_msg_1"

	deliveryRepo := pg.NewNotificationDeliveryRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	svc := &NotificationService{repo: notificationRepo, deliveryRepo: deliveryRepo}

	// --- Seed: two notifications.
	//   * `muted` + failure_reason=not_cataloged, provider_response NULL — the
	//     row the list must now be able to explain, and the nil-history case.
	//   * `delivered` with a two-event provider_response array — the timeline case.
	seedNotification := func(event string) int {
		var id int
		err := pool.QueryRow(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
			VALUES ($1, $2, '{}'::jsonb, 'digest', 'none', $3, 'delivered', now(), now())
			RETURNING id
		`, projectID, recipientExtID, event).Scan(&id)
		if err != nil {
			t.Fatalf("insert notification: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM notification WHERE id = $1", id) })
		return id
	}

	mutedNotificationID := seedNotification("muted-case")
	sentNotificationID := seedNotification("sent-case")

	_, err = pool.Exec(ctx, `
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, status, failure_reason, attempt, created_at, updated_at)
		VALUES ($1, $2, $3, 'email', 'muted', 'not_cataloged', 0, now(), now())
	`, mutedNotificationID, projectID, recipientExtID)
	if err != nil {
		t.Fatalf("insert muted delivery: %v", err)
	}

	// provider_response exactly as ApplyWebhookStatus builds it: a JSONB ARRAY of
	// raw Resend webhook bodies, appended one per event.
	events := `[
		{"type":"email.delivered","created_at":"2026-07-14T10:00:00.000Z","data":{"email_id":"dd_test_msg_1","to":["a@b.com"]}},
		{"type":"email.opened","created_at":"2026-07-14T10:05:00.000Z","data":{"email_id":"dd_test_msg_1"}}
	]`

	_, err = pool.Exec(ctx, `
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, contact_id, address_snapshot, status,
			 provider, provider_message_id, attempt, sent_at, delivered_at, opened_at, provider_response, created_at, updated_at)
		VALUES ($1, $2, $3, 'email', NULL, 'someone@example.com', 'delivered',
			 'resend', $4, 1, now(), now(), now(), $5::jsonb, now(), now())
	`, sentNotificationID, projectID, recipientExtID, providerMessageID, events)
	if err != nil {
		t.Fatalf("insert delivered delivery: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM notification_delivery WHERE notification_id = ANY($1)",
			[]int{mutedNotificationID, sentNotificationID})
	})

	// --- The LIST projection must carry failure_reason + the widened columns.
	// This is the Phase 9.1 bottleneck: before, it selected only status/sent_at/
	// delivered_at, so a `muted` row could not explain itself.
	t.Run("list projection carries the bounded delivery columns", func(t *testing.T) {
		notifications, _, err := notificationRepo.ListNotifications(ctx, projectID,
			enum.NotificationKindDirect, query.Pagination{Limit: 100, Page: 1})
		if err != nil {
			t.Fatalf("list notifications: %v", err)
		}

		var muted, sent *struct {
			status        enum.DeliveryStatus
			failureReason *string
			attempt       int
			address       *string
			openedAt      bool
		}

		for _, n := range notifications {
			if n.Email == nil {
				continue
			}
			row := &struct {
				status        enum.DeliveryStatus
				failureReason *string
				attempt       int
				address       *string
				openedAt      bool
			}{n.Email.Status, n.Email.FailureReason, n.Email.Attempt, n.Email.AddressSnapshot, n.Email.OpenedAt != nil}

			switch n.ID {
			case mutedNotificationID:
				muted = row
			case sentNotificationID:
				sent = row
			}
		}

		if muted == nil {
			t.Fatal("muted notification has no email delivery attached")
		}
		if muted.failureReason == nil || *muted.failureReason != "not_cataloged" {
			t.Errorf("muted failure_reason = %v, want not_cataloged — the list cannot explain the skip without it", muted.failureReason)
		}

		if sent == nil {
			t.Fatal("delivered notification has no email delivery attached")
		}
		if sent.status != enum.DeliveryStatus("delivered") {
			t.Errorf("status = %q, want delivered", sent.status)
		}
		if sent.attempt != 1 {
			t.Errorf("attempt = %d, want 1", sent.attempt)
		}
		if sent.address == nil || *sent.address != "someone@example.com" {
			t.Errorf("address_snapshot = %v, want someone@example.com", sent.address)
		}
		if !sent.openedAt {
			t.Error("opened_at not projected into the list")
		}
	})

	// --- The detail endpoint returns the history, normalized into a timeline.
	t.Run("detail returns normalized provider event timeline", func(t *testing.T) {
		result, errKind, err := svc.ListNotificationDeliveries(ctx, projectID, sentNotificationID)
		if err != nil || errKind != tantraService.ErrNone {
			t.Fatalf("list deliveries: errKind=%v err=%v", errKind, err)
		}
		if len(result.Deliveries) != 1 {
			t.Fatalf("got %d deliveries, want 1", len(result.Deliveries))
		}

		d := result.Deliveries[0]
		if d.ProviderMessageID == nil || *d.ProviderMessageID != providerMessageID {
			t.Errorf("provider_message_id = %v, want %s", d.ProviderMessageID, providerMessageID)
		}
		if len(d.Events) != 2 {
			t.Fatalf("got %d events, want 2", len(d.Events))
		}

		// Kind/At are normalized by the Resend adapter — the console must never
		// have to parse a provider's JSON shape itself.
		if d.Events[0].Kind != "delivered" {
			t.Errorf("event[0].kind = %q, want delivered", d.Events[0].Kind)
		}
		if d.Events[1].Kind != "opened" {
			t.Errorf("event[1].kind = %q, want opened", d.Events[1].Kind)
		}
		if d.Events[0].At == nil || d.Events[0].At.Format("15:04") != "10:00" {
			t.Errorf("event[0].at = %v, want the event's own 10:00 timestamp", d.Events[0].At)
		}
		// The verbatim event survives normalization.
		var raw map[string]any
		if err := json.Unmarshal(d.Events[1].Raw, &raw); err != nil {
			t.Fatalf("event raw is not valid JSON: %v", err)
		}
		if raw["type"] != "email.opened" {
			t.Errorf("raw type = %v, want email.opened", raw["type"])
		}
	})

	// --- A NULL provider_response (no webhook ever landed) must scan to no
	// events, not error. Every terminal-skip row in production looks like this.
	t.Run("null provider_response yields no events", func(t *testing.T) {
		result, errKind, err := svc.ListNotificationDeliveries(ctx, projectID, mutedNotificationID)
		if err != nil || errKind != tantraService.ErrNone {
			t.Fatalf("list deliveries: errKind=%v err=%v", errKind, err)
		}
		if len(result.Deliveries) != 1 {
			t.Fatalf("got %d deliveries, want 1", len(result.Deliveries))
		}
		if got := len(result.Deliveries[0].Events); got != 0 {
			t.Errorf("got %d events, want 0", got)
		}
		if r := result.Deliveries[0].FailureReason; r == nil || *r != "not_cataloged" {
			t.Errorf("failure_reason = %v, want not_cataloged", r)
		}
	})

	// --- Project scoping: the route only proves the caller owns the PROJECT, so
	// a notification id from another project must not resolve.
	t.Run("other project cannot read the delivery", func(t *testing.T) {
		const otherProjectID = 999999
		result, errKind, err := svc.ListNotificationDeliveries(ctx, otherProjectID, sentNotificationID)
		if err != nil || errKind != tantraService.ErrNone {
			t.Fatalf("list deliveries: errKind=%v err=%v", errKind, err)
		}
		if len(result.Deliveries) != 0 {
			t.Errorf("got %d deliveries for another project, want 0 — cross-project leak", len(result.Deliveries))
		}
	})
}
