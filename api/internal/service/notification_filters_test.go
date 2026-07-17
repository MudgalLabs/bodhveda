package service

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/tantra/query"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestNotificationFilters covers Phase 9.4's list filters against a real
// Postgres.
//
// The case that earns its keep is "in-app-only rows survive": the list attaches
// email deliveries with a SECOND batch query precisely because a notification
// with no email has no delivery row, and a join would drop it. The email filter
// therefore compiles to EXISTS, not a join — these tests pin that a filter only
// removes in-app-only rows when the operator explicitly asked about email, and
// that `email=none` can find them again.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestNotificationFilters(t *testing.T) {
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
		VALUES ($1, 'nf-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	for _, ext := range []string{"nf-alice", "nf-bob"} {
		_, err = pool.Exec(ctx, `
			INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
			VALUES ($1, 'NF', $2, now(), now())
		`, ext, projectID)
		if err != nil {
			t.Fatalf("insert recipient: %v", err)
		}
	}

	now := time.Now()
	seed := func(ext, channel, topic, event, status string, daysAgo int) int {
		var id int
		at := now.AddDate(0, 0, -daysAgo)
		err := pool.QueryRow(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, broadcast_id,
				channel, topic, event, status, created_at, updated_at)
			VALUES ($1, $2, '{}'::jsonb, NULL, $3, $4, $5, $6, $7, $7)
			RETURNING id
		`, projectID, ext, channel, topic, event, status, at).Scan(&id)
		if err != nil {
			t.Fatalf("insert notification: %v", err)
		}
		return id
	}

	seedEmail := func(notificationID int, ext, status, failureReason string) {
		var fr *string
		if failureReason != "" {
			fr = &failureReason
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO notification_delivery
				(notification_id, project_id, recipient_external_id, medium, status, failure_reason,
				 attempt, created_at, updated_at)
			VALUES ($1, $2, $3, 'email', $4, $5, 1, now(), now())
		`, notificationID, projectID, ext, status, fr)
		if err != nil {
			t.Fatalf("insert delivery: %v", err)
		}
	}

	// The fixture, chosen so every filter has both a match and a non-match:
	//
	//  id        recipient  target                  status     age   email
	//  inAppOnly nf-alice   digest/none/sent        delivered  1d    (none)  <- must survive
	//  bounced   nf-alice   digest/none/sent        delivered  2d    bounced
	//  mutedPref nf-bob     digest/none/sent        muted      3d    muted/preference_disabled
	//  otherTgt  nf-bob     posts/post_1/new_comment delivered 40d   sent
	inAppOnly := seed("nf-alice", "digest", "none", "sent", "delivered", 1)
	bounced := seed("nf-alice", "digest", "none", "sent", "delivered", 2)
	seedEmail(bounced, "nf-alice", "bounced", "")
	mutedPref := seed("nf-bob", "digest", "none", "sent", "muted", 3)
	seedEmail(mutedPref, "nf-bob", "muted", "preference_disabled")
	otherTgt := seed("nf-bob", "posts", "post_1", "new_comment", "delivered", 40)
	seedEmail(otherTgt, "nf-bob", "sent", "")

	svc := &NotificationService{repo: pg.NewNotificationRepo(pool)}

	list := func(t *testing.T, f *dto.ListNotificationsFilters) []int {
		t.Helper()
		f.ProjectID = projectID
		f.Pagination = query.Pagination{Limit: 100, Page: 1}
		res, errKind, err := svc.ListNotifications(ctx, f)
		if err != nil {
			t.Fatalf("ListNotifications: %v (%v)", err, errKind)
		}
		ids := make([]int, 0, len(res.Notifications))
		for _, n := range res.Notifications {
			ids = append(ids, n.ID)
		}
		// The paginated total must agree with the filtered rows, or the pager
		// offers pages that render empty.
		if res.Pagination.TotalItems != len(ids) {
			t.Errorf("total_items = %d but got %d rows — the COUNT lost the filters",
				res.Pagination.TotalItems, len(ids))
		}
		return ids
	}

	str := func(s string) *string { return &s }

	t.Run("no filters returns every direct notification", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{})
		if len(got) != 4 {
			t.Fatalf("got %v, want all 4", got)
		}
	})

	// THE point of the EXISTS-not-JOIN design.
	t.Run("in-app-only rows survive filters that are not about email", func(t *testing.T) {
		for name, f := range map[string]*dto.ListNotificationsFilters{
			"status":    {Status: ptr(enum.NotificationStatusDelivered)},
			"target":    {Channel: str("digest"), Topic: str("none"), Event: str("sent")},
			"recipient": {RecipientExtID: str("nf-alice")},
			"date":      {CreatedFrom: ptr(now.AddDate(0, 0, -7))},
		} {
			t.Run(name, func(t *testing.T) {
				got := list(t, f)
				if !contains(got, inAppOnly) {
					t.Errorf("%s filter dropped the in-app-only notification %d (got %v) — "+
						"a JOIN would do exactly this", name, inAppOnly, got)
				}
			})
		}
	})

	t.Run("email=none finds exactly the in-app-only rows", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{Email: ptr(enum.EmailFilterNone)})
		if len(got) != 1 || got[0] != inAppOnly {
			t.Fatalf("got %v, want [%d] — the only send that carried no email", got, inAppOnly)
		}
	})

	t.Run("email=any excludes the in-app-only row", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{Email: ptr(enum.EmailFilterAny)})
		if len(got) != 3 || contains(got, inAppOnly) {
			t.Fatalf("got %v, want the 3 that attempted email", got)
		}
	})

	t.Run("email=<status> selects on the delivery row, not the notification", func(t *testing.T) {
		// `bounced` is deliberately a notification whose OWN status is
		// `delivered`: in-app succeeded, email bounced. Filtering on the email
		// status must not be confusable with filtering on the in-app one.
		got := list(t, &dto.ListNotificationsFilters{Email: ptr(enum.EmailDeliveryFilter(enum.DeliveryBounced))})
		if len(got) != 1 || got[0] != bounced {
			t.Fatalf("got %v, want [%d]", got, bounced)
		}

		got = list(t, &dto.ListNotificationsFilters{Status: ptr(enum.NotificationStatusDelivered)})
		if !contains(got, bounced) {
			t.Errorf("in-app status=delivered lost %d, whose email bounced", bounced)
		}
	})

	t.Run("the brief's question: bounced email on digest/none/sent last week", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{
			Channel:     str("digest"),
			Topic:       str("none"),
			Event:       str("sent"),
			Email:       ptr(enum.EmailDeliveryFilter(enum.DeliveryBounced)),
			CreatedFrom: ptr(now.AddDate(0, 0, -7)),
		})
		if len(got) != 1 || got[0] != bounced {
			t.Fatalf("got %v, want [%d]", got, bounced)
		}
	})

	t.Run("date range is inclusive and bounded on both ends", func(t *testing.T) {
		// otherTgt is 40 days old; everything else is within 3 days.
		got := list(t, &dto.ListNotificationsFilters{CreatedFrom: ptr(now.AddDate(0, 0, -7))})
		if contains(got, otherTgt) {
			t.Errorf("created_from let the 40-day-old row %d through (got %v)", otherTgt, got)
		}

		got = list(t, &dto.ListNotificationsFilters{CreatedTo: ptr(now.AddDate(0, 0, -7))})
		if len(got) != 1 || got[0] != otherTgt {
			t.Errorf("created_to = %v, want only the 40-day-old row %d", got, otherTgt)
		}
	})

	t.Run("recipient_search is a substring; recipient_id is exact", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{RecipientSearch: str("ALI")})
		if len(got) != 2 {
			t.Errorf("substring search for ALI (case-insensitive) = %v, want alice's 2", got)
		}

		got = list(t, &dto.ListNotificationsFilters{RecipientExtID: str("nf-ali")})
		if len(got) != 0 {
			t.Errorf("exact match on a prefix = %v, want none", got)
		}
	})

	// External ids are customer-chosen and very often contain `_`, which LIKE
	// reads as "any single character" unless escaped.
	t.Run("recipient_search treats LIKE wildcards as literal text", func(t *testing.T) {
		if got := list(t, &dto.ListNotificationsFilters{RecipientSearch: str("nf_alice")}); len(got) != 0 {
			t.Errorf("search for the literal `nf_alice` = %v, want none — `_` must not match the `-` in nf-alice", got)
		}
		if got := list(t, &dto.ListNotificationsFilters{RecipientSearch: str("%")}); len(got) != 0 {
			t.Errorf("search for a literal `%%` = %v, want none — no id contains one", got)
		}
		// And the escaping must not break an ordinary search.
		if got := list(t, &dto.ListNotificationsFilters{RecipientSearch: str("nf-alice")}); len(got) != 2 {
			t.Errorf("search for `nf-alice` = %v, want alice's 2", got)
		}
	})

	t.Run("filters compose (AND, not OR)", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{
			Status:         ptr(enum.NotificationStatusMuted),
			RecipientExtID: str("nf-bob"),
			Channel:        str("digest"),
		})
		if len(got) != 1 || got[0] != mutedPref {
			t.Fatalf("got %v, want [%d]", got, mutedPref)
		}
	})

	// A filter naming something that cannot exist is a typo, not a query.
	t.Run("an impossible filter value is rejected, not silently empty", func(t *testing.T) {
		for name, f := range map[string]*dto.ListNotificationsFilters{
			"status": {Status: ptr(enum.NotificationStatus("bogus"))},
			"email":  {Email: ptr(enum.EmailDeliveryFilter("bogus"))},
			"kind":   {Kind: enum.NotificationKind("bogus")},
			"range":  {CreatedFrom: ptr(now), CreatedTo: ptr(now.AddDate(0, 0, -1))},
		} {
			t.Run(name, func(t *testing.T) {
				f.ProjectID = projectID
				f.Pagination = query.Pagination{Limit: 10, Page: 1}
				_, errKind, err := svc.ListNotifications(ctx, f)
				if err == nil {
					t.Fatalf("expected a validation error for a bogus %s", name)
				}
				if errKind != tantraService.ErrInvalidInput {
					t.Errorf("errKind = %v, want ErrInvalidInput", errKind)
				}
			})
		}
	})

	t.Run("blank filters are absent, not a filter on the empty string", func(t *testing.T) {
		// A UI clearing a control can send `?channel=`; that must not mean
		// "channel equals empty string", which matches nothing.
		got := list(t, &dto.ListNotificationsFilters{
			Channel:         str(""),
			RecipientExtID:  str("  "),
			RecipientSearch: str(""),
			Status:          ptr(enum.NotificationStatus("")),
			Email:           ptr(enum.EmailDeliveryFilter("")),
		})
		if len(got) != 4 {
			t.Fatalf("got %v, want all 4 — blank filters must not narrow", got)
		}
	})

	t.Run("external id filters are lowercased to match storage", func(t *testing.T) {
		got := list(t, &dto.ListNotificationsFilters{RecipientExtID: str("NF-ALICE")})
		if len(got) != 2 {
			t.Errorf("got %v, want alice's 2 — external ids are stored lowercase", got)
		}
	})

	t.Run("filters are scoped to the project", func(t *testing.T) {
		var otherProject int
		err := pool.QueryRow(ctx, `
			INSERT INTO project (user_id, name, created_at, updated_at)
			VALUES ($1, 'nf-test-other', now(), now()) RETURNING id
		`, userID).Scan(&otherProject)
		if err != nil {
			t.Fatalf("insert project: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", otherProject) })

		res, _, err := svc.ListNotifications(ctx, &dto.ListNotificationsFilters{
			ProjectID:  otherProject,
			Email:      ptr(enum.EmailDeliveryFilter(enum.DeliveryBounced)),
			Pagination: query.Pagination{Limit: 100, Page: 1},
		})
		if err != nil {
			t.Fatalf("ListNotifications: %v", err)
		}
		if len(res.Notifications) != 0 {
			t.Errorf("got %d rows, want 0 — the EXISTS subquery must not reach across projects",
				len(res.Notifications))
		}
	})
}

func ptr[T any](v T) *T { return &v }

func contains(haystack []int, needle int) bool {
	return slices.Contains(haystack, needle)
}
