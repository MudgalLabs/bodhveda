package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/tantra/query"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestRecipientDetail covers the two reads Phase 9.2's recipient detail page is
// built on, against a real Postgres.
//
// The cross-project cases are the point: `recipient.external_id` is unique only
// WITHIN a project, so a customer-chosen id like "user_1" legitimately exists in
// several projects at once. Both reads must refuse to blend them.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestRecipientDetail(t *testing.T) {
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

	// The SAME external id in two projects — the collision the production schema
	// permits and the old unscoped JOIN silently merged.
	const sharedExtID = "rd-test-user-1"

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	seedProject := func(name string) int {
		var id int
		err := pool.QueryRow(ctx, `
			INSERT INTO project (user_id, name, created_at, updated_at)
			VALUES ($1, $2, now(), now()) RETURNING id
		`, userID, name).Scan(&id)
		if err != nil {
			t.Fatalf("insert project: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", id) })
		return id
	}

	projectA := seedProject("rd-test-A")
	projectB := seedProject("rd-test-B")

	for _, pid := range []int{projectA, projectB} {
		_, err = pool.Exec(ctx, `
			INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
			VALUES ($1, 'RD Test', $2, now(), now())
		`, sharedExtID, pid)
		if err != nil {
			t.Fatalf("insert recipient: %v", err)
		}
	}

	// Notification mix. Project A: 2 direct (one muted) + 1 broadcast.
	// Project B: 5 direct for the SAME external id — none may leak into A.
	seedNotification := func(projectID int, status string, broadcast bool) int {
		var broadcastID *int
		if broadcast {
			var bid int
			err := pool.QueryRow(ctx, `
				INSERT INTO broadcast (project_id, payload, channel, topic, event, status, created_at, updated_at)
				VALUES ($1, '{}'::jsonb, 'digest', 'none', 'sent', 'completed', now(), now())
				RETURNING id
			`, projectID).Scan(&bid)
			if err != nil {
				t.Fatalf("insert broadcast: %v", err)
			}
			broadcastID = &bid
		}

		var id int
		err := pool.QueryRow(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, status, created_at, updated_at)
			VALUES ($1, $2, '{}'::jsonb, $3, 'digest', 'none', 'sent', $4, now(), now())
			RETURNING id
		`, projectID, sharedExtID, broadcastID, status).Scan(&id)
		if err != nil {
			t.Fatalf("insert notification: %v", err)
		}
		return id
	}

	mutedInA := seedNotification(projectA, "muted", false)
	seedNotification(projectA, "delivered", false)
	seedNotification(projectA, "delivered", true)
	for range 5 {
		seedNotification(projectB, "delivered", false)
	}

	// An email delivery on A's muted notification: the recipient feed must carry
	// it so 9.1's per-medium status cell + detail dialog have something to render.
	_, err = pool.Exec(ctx, `
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, status, failure_reason, attempt, created_at, updated_at)
		VALUES ($1, $2, $3, 'email', 'muted', 'preference_disabled', 0, now(), now())
	`, mutedInA, projectA, sharedExtID)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}

	recipientRepo := pg.NewRecipientRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	recipientSvc := &RecipientService{repo: recipientRepo}
	notificationSvc := &NotificationService{repo: notificationRepo}

	t.Run("counts are scoped to the project", func(t *testing.T) {
		got, errKind, err := recipientSvc.GetWithCounts(ctx, projectA, sharedExtID)
		if err != nil {
			t.Fatalf("GetWithCounts: %v (%v)", err, errKind)
		}

		// Project B's 5 direct notifications share this external id. Before the
		// join was project-scoped, they were counted here.
		if got.DirectNotificationsCount != 2 {
			t.Errorf("direct count = %d, want 2 (project B's 5 must not leak in)", got.DirectNotificationsCount)
		}
		if got.BroadcastNotificationsCount != 1 {
			t.Errorf("broadcast count = %d, want 1", got.BroadcastNotificationsCount)
		}
		if got.ExternalID != sharedExtID {
			t.Errorf("external id = %q, want %q", got.ExternalID, sharedExtID)
		}
	})

	t.Run("unknown recipient is not found", func(t *testing.T) {
		_, errKind, err := recipientSvc.GetWithCounts(ctx, projectA, "rd-test-nobody")
		if err == nil {
			t.Fatal("expected an error for an unknown recipient")
		}
		if errKind != tantraService.ErrNotFound {
			t.Errorf("errKind = %v, want ErrNotFound", errKind)
		}
	})

	t.Run("another project's recipient is not found", func(t *testing.T) {
		// projectB's recipient exists, but not under a project the caller named.
		unrelated := seedProject("rd-test-C")
		_, errKind, err := recipientSvc.GetWithCounts(ctx, unrelated, sharedExtID)
		if err == nil {
			t.Fatal("expected an error: the id exists, but not in this project")
		}
		if errKind != tantraService.ErrNotFound {
			t.Errorf("errKind = %v, want ErrNotFound", errKind)
		}
	})

	// The feed is the operator's view, NOT the recipient's inbox: it must keep
	// `muted` rows (ListForRecipient strips them) and carry the email outcome.
	t.Run("feed keeps muted rows and carries the email delivery", func(t *testing.T) {
		res, errKind, err := notificationSvc.ListNotifications(ctx, &dto.ListNotificationsFilters{
			ProjectID:      projectA,
			Kind:           enum.NotificationKindAll,
			RecipientExtID: &[]string{sharedExtID}[0],
			Pagination:     query.Pagination{Limit: 100, Page: 1},
		})
		if err != nil {
			t.Fatalf("ListNotifications: %v (%v)", err, errKind)
		}

		if len(res.Notifications) != 3 {
			t.Fatalf("got %d notifications, want 3 (2 direct + 1 broadcast, project A only)",
				len(res.Notifications))
		}

		var muted *dto.Notification
		for _, n := range res.Notifications {
			if n.ID == mutedInA {
				muted = n
			}
		}
		if muted == nil {
			t.Fatal("the muted notification is missing — the operator feed must not hide it")
		}
		if muted.Email == nil {
			t.Fatal("email delivery not attached; 9.1's status cell would render nothing")
		}
		if muted.Email.FailureReason == nil || *muted.Email.FailureReason != "preference_disabled" {
			t.Errorf("failure_reason = %v, want preference_disabled", muted.Email.FailureReason)
		}
	})

	t.Run("feed is scoped to the project", func(t *testing.T) {
		res, _, err := notificationSvc.ListNotifications(ctx, &dto.ListNotificationsFilters{
			ProjectID:      projectB,
			Kind:           enum.NotificationKindAll,
			RecipientExtID: &[]string{sharedExtID}[0],
			Pagination:     query.Pagination{Limit: 100, Page: 1},
		})
		if err != nil {
			t.Fatalf("ListNotifications: %v", err)
		}
		if len(res.Notifications) != 5 {
			t.Errorf("got %d notifications for project B, want 5 (A's must not leak in)",
				len(res.Notifications))
		}
	})

	t.Run("kind still defaults to direct when omitted", func(t *testing.T) {
		// The project Notifications list relies on this default; the new `all`
		// value must not have changed it.
		res, _, err := notificationSvc.ListNotifications(ctx, &dto.ListNotificationsFilters{
			ProjectID:      projectA,
			RecipientExtID: &[]string{sharedExtID}[0],
			Pagination:     query.Pagination{Limit: 100, Page: 1},
		})
		if err != nil {
			t.Fatalf("ListNotifications: %v", err)
		}
		if len(res.Notifications) != 2 {
			t.Errorf("got %d notifications, want 2 direct-only", len(res.Notifications))
		}
	})
}
