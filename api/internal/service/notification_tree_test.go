package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestNotificationDeliveryTree covers the DIRECT-send delivery tree — the same
// dto.DeliveryTree shape the broadcast tree returns, because a direct send is
// that tree with a fan-out of one. Keeping one shape is what makes the two
// comparable in the console rather than two unrelated screens.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestNotificationDeliveryTree(t *testing.T) {
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
		t.Fatalf("borrow user_id: %v", err)
	}

	var projectID int
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'direct-tree-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	svc := &NotificationService{
		repo:         pg.NewNotificationRepo(pool),
		deliveryRepo: pg.NewNotificationDeliveryRepo(pool),
	}

	seed := func(status enum.NotificationStatus) int {
		t.Helper()
		var id int
		if err := pool.QueryRow(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, channel,
				topic, event, created_at, updated_at, status)
			VALUES ($1, 'r1', '{"t":"hi"}'::jsonb, 'product', 'updates', 'released', now(), now(), $2)
			RETURNING id
		`, projectID, string(status)).Scan(&id); err != nil {
			t.Fatalf("seed notification: %v", err)
		}
		return id
	}

	t.Run("in-app only send has exactly one branch", func(t *testing.T) {
		// An absent email branch and an email branch with zero in it are different
		// facts: this send never asked for email, so there must be no email row.
		id := seed(enum.NotificationStatusDelivered)

		tree, errKind, err := svc.GetDeliveryTree(ctx, projectID, id)
		if err != nil {
			t.Fatalf("get tree: %v (%v)", err, errKind)
		}

		if tree.Kind != enum.NotificationKindDirect {
			t.Errorf("kind = %q, want direct", tree.Kind)
		}
		// ⚠️ No Audience node on a direct send — it names its recipient, so there
		// is no audience to resolve. A zeroed audience would be a lie.
		if tree.Audience != nil {
			t.Errorf("a direct send must have no audience node, got %+v", tree.Audience)
		}
		if len(tree.Mediums) != 1 {
			t.Fatalf("want 1 medium branch, got %d", len(tree.Mediums))
		}

		m := tree.Mediums[0]
		if m.Medium != string(enum.MediumInApp) || m.Total != 1 {
			t.Errorf("branch = %+v, want in_app total 1", m)
		}
		if got := m.Outcomes[string(enum.OutcomeSucceeded)]; got != 1 {
			t.Errorf("succeeded = %d, want 1", got)
		}
	})

	t.Run("email-only send reports not_requested for in-app", func(t *testing.T) {
		// `not_requested` must be its own bucket: the SENDER never asked for in-app.
		// Counting it as succeeded would inflate delivery; as failed would invent a
		// problem.
		id := seed(enum.NotificationStatusNotRequested)

		tree, _, err := svc.GetDeliveryTree(ctx, projectID, id)
		if err != nil {
			t.Fatalf("get tree: %v", err)
		}

		m := tree.Mediums[0]
		if got := m.Outcomes[string(enum.OutcomeNotRequested)]; got != 1 {
			t.Errorf("not_requested = %d, want 1", got)
		}
		if m.Outcomes[string(enum.OutcomeSucceeded)] != 0 || m.Outcomes[string(enum.OutcomeFailed)] != 0 {
			t.Errorf("not_requested must be neither success nor failure, got %v", m.Outcomes)
		}
	})

	t.Run("muted in-app is suppressed, not failed", func(t *testing.T) {
		id := seed(enum.NotificationStatusMuted)

		tree, _, err := svc.GetDeliveryTree(ctx, projectID, id)
		if err != nil {
			t.Fatalf("get tree: %v", err)
		}

		m := tree.Mediums[0]
		if got := m.Outcomes[string(enum.OutcomeSuppressed)]; got != 1 {
			t.Errorf("suppressed = %d, want 1", got)
		}
		if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 0 {
			t.Errorf("an opted-out recipient must not count as a failure, got %d", got)
		}
	})

	t.Run("email branch appears when the send attempted email", func(t *testing.T) {
		id := seed(enum.NotificationStatusDelivered)

		if _, err := pool.Exec(ctx, `
			INSERT INTO notification_delivery (notification_id, project_id,
				recipient_external_id, medium, status, failure_reason, attempt, created_at, updated_at)
			VALUES ($1, $2, 'r1', 'email', 'bounced', 'provider_send_error', 1, now(), now())
		`, id, projectID); err != nil {
			t.Fatalf("seed delivery: %v", err)
		}

		tree, _, err := svc.GetDeliveryTree(ctx, projectID, id)
		if err != nil {
			t.Fatalf("get tree: %v", err)
		}

		if len(tree.Mediums) != 2 {
			t.Fatalf("want in_app + email branches, got %d", len(tree.Mediums))
		}

		// The diverging-outcome case the shared shape exists for: in-app delivered,
		// email bounced. One notification, two different answers.
		inApp, email := tree.Mediums[0], tree.Mediums[1]
		if got := inApp.Outcomes[string(enum.OutcomeSucceeded)]; got != 1 {
			t.Errorf("in_app succeeded = %d, want 1", got)
		}
		if email.Medium != string(enum.MediumEmail) {
			t.Errorf("second branch = %q, want email", email.Medium)
		}
		if got := email.Outcomes[string(enum.OutcomeFailed)]; got != 1 {
			t.Errorf("bounced email should be a failure, got %d", got)
		}
		if email.Pending != 0 {
			t.Errorf("bounced is terminal, got pending %d", email.Pending)
		}
	})

	t.Run("cross-project access 404s", func(t *testing.T) {
		id := seed(enum.NotificationStatusDelivered)

		tree, errKind, err := svc.GetDeliveryTree(ctx, projectID+100000, id)
		if err == nil {
			t.Fatal("reading another project's notification must fail")
		}
		if tree != nil {
			t.Error("no tree should be returned")
		}
		if errKind != tantraService.ErrNotFound {
			t.Errorf("errKind = %v, want ErrNotFound", errKind)
		}
	})
}
