package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/pg"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestProjectAnalytics covers Phase 9.5's analytics aggregate against a real
// Postgres.
//
// The invariant that earns its keep is the same one 9.4 pinned from the other
// side: in-app and email outcomes live in DIFFERENT tables, so they are two
// aggregates, never one join. An in-app-only notification (no email block, no
// delivery row) must be counted in InApp but absent from Email — a naive join
// would drop it, and it is still the common case. The fixture makes that row the
// majority.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestProjectAnalytics(t *testing.T) {
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
		VALUES ($1, 'analytics-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ('an-rec', 'AN', $1, now(), now())
	`, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	// Bucket everything into a single fixed instant so the date-range and
	// day-bucket assertions are deterministic regardless of when the test runs.
	// 2026-06-15 18:30:00 UTC — chosen because under Asia/Kolkata (+05:30) it is
	// 2026-06-16 00:00, i.e. it crosses the local day boundary. That is the whole
	// point of bucketing in the viewer's timezone.
	at := time.Date(2026, 6, 15, 18, 30, 0, 0, time.UTC)

	seed := func(channel, topic, event, status string) int {
		var id int
		err := pool.QueryRow(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, broadcast_id,
				channel, topic, event, status, created_at, updated_at)
			VALUES ($1, 'an-rec', '{}'::jsonb, NULL, $2, $3, $4, $5, $6, $6)
			RETURNING id
		`, projectID, channel, topic, event, status, at).Scan(&id)
		if err != nil {
			t.Fatalf("insert notification: %v", err)
		}
		return id
	}
	seedEmail := func(notificationID int, status string, openedAt *time.Time) {
		_, err := pool.Exec(ctx, `
			INSERT INTO notification_delivery
				(notification_id, project_id, recipient_external_id, medium, status,
				 attempt, opened_at, created_at, updated_at)
			VALUES ($1, $2, 'an-rec', 'email', $3, 1, $4, $5, $5)
		`, notificationID, projectID, status, openedAt, at)
		if err != nil {
			t.Fatalf("insert delivery: %v", err)
		}
	}

	// Fixture:
	//   target digest/none/sent : 3 in-app (2 delivered, 1 muted); 2 email (delivered+opened, bounced)
	//   target posts/p1/comment : 1 in-app delivered; 0 email  <- in-app-only, must not appear in Email
	d1 := seed("digest", "none", "sent", "delivered")
	seed("digest", "none", "sent", "delivered")
	seed("digest", "none", "sent", "muted")
	d2 := seed("digest", "none", "sent", "delivered")
	seed("posts", "p1", "comment", "delivered")

	opened := at.Add(2 * time.Hour)
	seedEmail(d1, "delivered", &opened)
	seedEmail(d2, "bounced", nil)

	svc := &NotificationService{
		repo:         pg.NewNotificationRepo(pool),
		deliveryRepo: pg.NewNotificationDeliveryRepo(pool),
	}

	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC)

	get := func(t *testing.T, tz string) *dto.ProjectAnalytics {
		t.Helper()
		res, errKind, err := svc.ProjectAnalytics(ctx, &dto.AnalyticsFilters{
			ProjectID:   projectID,
			CreatedFrom: &from,
			CreatedTo:   &to,
		}, tz)
		if err != nil {
			t.Fatalf("ProjectAnalytics: %v (%v)", err, errKind)
		}
		return res
	}

	t.Run("in-app aggregates the notification row, all rows counted", func(t *testing.T) {
		r := get(t, "UTC")
		if r.InApp.Total != 5 {
			t.Fatalf("InApp.Total = %d, want 5", r.InApp.Total)
		}
		if r.InApp.ByStatus.Delivered != 4 {
			t.Errorf("InApp delivered = %d, want 4", r.InApp.ByStatus.Delivered)
		}
		if r.InApp.ByStatus.Muted != 1 {
			t.Errorf("InApp muted = %d, want 1", r.InApp.ByStatus.Muted)
		}
	})

	t.Run("email aggregates the delivery table; in-app-only rows absent", func(t *testing.T) {
		r := get(t, "UTC")
		// Only the 2 email deliveries — NOT the 5 notifications. This is the
		// "different tables, no join" invariant: the 3 in-app-only rows (including
		// posts/p1/comment) do not leak into the email total.
		if r.Email.Attempted != 2 {
			t.Fatalf("Email.Attempted = %d, want 2 (a join would inflate this)", r.Email.Attempted)
		}
		if r.Email.ByStatus.Delivered != 1 || r.Email.ByStatus.Bounced != 1 {
			t.Errorf("Email by_status delivered=%d bounced=%d, want 1/1",
				r.Email.ByStatus.Delivered, r.Email.ByStatus.Bounced)
		}
		// opened is a soft signal counted from opened_at, not a status.
		if r.Email.Opened != 1 {
			t.Errorf("Email.Opened = %d, want 1", r.Email.Opened)
		}
	})

	t.Run("per-target breakdown merges in-app volume with email stats", func(t *testing.T) {
		r := get(t, "UTC")
		byTarget := map[string]dto.AnalyticsTargetStat{}
		for _, tg := range r.Targets {
			byTarget[tg.Channel+"/"+tg.Topic+"/"+tg.Event] = tg
		}

		digest, ok := byTarget["digest/none/sent"]
		if !ok {
			t.Fatal("digest/none/sent missing from targets")
		}
		if digest.Notifications != 4 {
			t.Errorf("digest notifications = %d, want 4", digest.Notifications)
		}
		if digest.EmailAttempted != 2 || digest.EmailBounced != 1 {
			t.Errorf("digest email attempted=%d bounced=%d, want 2/1",
				digest.EmailAttempted, digest.EmailBounced)
		}

		posts, ok := byTarget["posts/p1/comment"]
		if !ok {
			t.Fatal("posts/p1/comment (in-app-only target) missing from targets")
		}
		if posts.Notifications != 1 {
			t.Errorf("posts notifications = %d, want 1", posts.Notifications)
		}
		if posts.EmailAttempted != 0 {
			t.Errorf("posts email attempted = %d, want 0 (never sent email)", posts.EmailAttempted)
		}

		// Most-active target first.
		if len(r.Targets) < 1 || r.Targets[0].Channel != "digest" {
			t.Errorf("targets not ordered by volume; first = %+v", r.Targets[0])
		}
	})

	t.Run("day bucketing respects the viewer timezone", func(t *testing.T) {
		// The seed instant is 2026-06-15 18:30 UTC == 2026-06-16 00:00 +05:30.
		utc := get(t, "UTC")
		if len(utc.InApp.Series) != 1 || utc.InApp.Series[0].Day != "2026-06-15" {
			t.Fatalf("UTC series = %+v, want single day 2026-06-15", utc.InApp.Series)
		}
		kolkata := get(t, "Asia/Kolkata")
		if len(kolkata.InApp.Series) != 1 || kolkata.InApp.Series[0].Day != "2026-06-16" {
			t.Fatalf("Kolkata series = %+v, want single day 2026-06-16", kolkata.InApp.Series)
		}
	})

	t.Run("date range excludes rows outside it", func(t *testing.T) {
		// A window entirely before the seed instant returns nothing, not an error.
		before := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		beforeEnd := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
		res, _, err := svc.ProjectAnalytics(ctx, &dto.AnalyticsFilters{
			ProjectID:   projectID,
			CreatedFrom: &before,
			CreatedTo:   &beforeEnd,
		}, "UTC")
		if err != nil {
			t.Fatalf("ProjectAnalytics: %v", err)
		}
		if res.InApp.Total != 0 || res.Email.Attempted != 0 || len(res.Targets) != 0 {
			t.Errorf("out-of-range window returned data: inApp=%d email=%d targets=%d",
				res.InApp.Total, res.Email.Attempted, len(res.Targets))
		}
	})

	t.Run("inverted range is a 400, not empty data", func(t *testing.T) {
		_, errKind, err := svc.ProjectAnalytics(ctx, &dto.AnalyticsFilters{
			ProjectID:   projectID,
			CreatedFrom: &to,
			CreatedTo:   &from,
		}, "UTC")
		if err == nil {
			t.Fatal("expected an error for an inverted range")
		}
		if errKind != tantraService.ErrInvalidInput {
			t.Errorf("errKind = %v, want ErrInvalidInput", errKind)
		}
	})
}
