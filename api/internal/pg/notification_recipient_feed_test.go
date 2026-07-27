package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

// TestRecipientFeedExcludesUndeliveredStatuses pins the three recipient-scoped
// read paths to ONE visibility rule (recipientFeedVisible), against a live
// Postgres.
//
// They are tested together on purpose. The failure mode that matters is not any
// one query being wrong — it is the three DRIFTING APART: a feed that hides a
// row while the unread count still counts it produces a badge the user can never
// clear, and mark-all-read silently stamping hidden rows makes an email-only
// notification look like something the recipient read.
//
// The `not_requested` row also doubles as proof the payload-nullable migration
// landed: it is inserted with a NULL payload, which the pre-migration schema
// rejects outright.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestRecipientFeedExcludesUndeliveredStatuses(t *testing.T) {
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

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	err = pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'feed-visibility-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	const extID = "feed-user"
	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, 'Feed', $2, now(), now())
	`, extID, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	// One notification per status. `not_requested` carries a NULL payload, exactly
	// as an email-only send stores it; every other row carries a real one.
	type seed struct {
		status  enum.NotificationStatus
		payload any
		visible bool
	}
	seeds := []seed{
		{enum.NotificationStatusDelivered, `{"n":"delivered"}`, true},
		{enum.NotificationStatusEnqueued, `{"n":"enqueued"}`, true},
		{enum.NotificationStatusMuted, `{"n":"muted"}`, false},
		{enum.NotificationStatusQuotaExceeded, `{"n":"quota"}`, false},
		{enum.NotificationStatusNotRequested, nil, false},
	}

	idByStatus := make(map[enum.NotificationStatus]int, len(seeds))
	wantVisible := 0
	for _, s := range seeds {
		var id int
		err := pool.QueryRow(ctx, `
			INSERT INTO notification
				(project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'conversation', 'thread_7', 'reply', $4, now(), now())
			RETURNING id
		`, projectID, extID, s.payload, string(s.status)).Scan(&id)
		if err != nil {
			// A NULL payload failing here means the payload-nullable migration has
			// not been applied to this database.
			t.Fatalf("insert %s notification (payload=%v): %v", s.status, s.payload, err)
		}
		idByStatus[s.status] = id
		if s.visible {
			wantVisible++
		}
	}

	repo := NewNotificationRepo(pool)

	// 1. The feed shows only the delivered/in-flight rows.
	limit := 50
	notifs, _, err := repo.ListForRecipient(ctx, projectID, extID, &query.Cursor{Limit: &limit})
	if err != nil {
		t.Fatalf("list for recipient: %v", err)
	}
	if len(notifs) != wantVisible {
		got := make([]enum.NotificationStatus, 0, len(notifs))
		for _, n := range notifs {
			got = append(got, n.Status)
		}
		t.Errorf("feed returned %d rows %v, want %d (delivered + enqueued only)", len(notifs), got, wantVisible)
	}
	for _, n := range notifs {
		switch n.Status {
		case enum.NotificationStatusMuted, enum.NotificationStatusQuotaExceeded, enum.NotificationStatusNotRequested:
			t.Errorf("notification %d with status %s leaked into the recipient feed", n.ID, n.Status)
		}
	}

	// 2. The unread count agrees with the feed. None of the seeds are read yet, so
	//    "unread" and "visible" are the same set here — which is the point.
	count, err := repo.UnreadCountForRecipient(ctx, projectID, extID)
	if err != nil {
		t.Fatalf("unread count: %v", err)
	}
	if count != wantVisible {
		t.Errorf("unread count = %d, want %d — the badge and the feed disagree, so the user sees a count they cannot clear", count, wantVisible)
	}

	// 3. Mark-all-read touches only the visible rows.
	readTrue := true
	updated, err := repo.UpdateForRecipient(ctx, projectID, extID, dto.UpdateRecipientNotificationsPayload{
		State: dto.NotificationStateFilter{Read: &readTrue},
	})
	if err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if updated != wantVisible {
		t.Errorf("mark-all-read updated %d rows, want %d", updated, wantVisible)
	}

	for _, s := range seeds {
		if s.visible {
			continue
		}
		var readAt *string
		err := pool.QueryRow(ctx, `SELECT read_at::text FROM notification WHERE id = $1`, idByStatus[s.status]).Scan(&readAt)
		if err != nil {
			t.Fatalf("read back %s notification: %v", s.status, err)
		}
		if readAt != nil {
			t.Errorf("%s notification was marked read (%s) — the recipient was never shown it", s.status, *readAt)
		}
	}

	// 4. And the count is now zero, not merely smaller.
	count, err = repo.UnreadCountForRecipient(ctx, projectID, extID)
	if err != nil {
		t.Fatalf("unread count after mark-all-read: %v", err)
	}
	if count != 0 {
		t.Errorf("unread count after mark-all-read = %d, want 0 — a count that cannot be cleared", count)
	}
}
