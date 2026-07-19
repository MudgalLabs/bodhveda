package service

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
)

// TestResolveRecipientPreferencesAgreesWithGating is the load-bearing test of
// Phase 9.3.
//
// The console's preference grid resolves a recipient's (target, medium) cells
// with a SECOND SQL resolver (PreferenceRepo.ResolveRecipientPreferences) —
// set-based, so the grid costs one round trip instead of one per cell. The send
// path keeps its own single-cell resolver
// (PreferenceRepo.ShouldDirectNotificationBeDelivered) because it sits on the
// hot path. Two implementations of one cascade is the cost of that choice, and
// this test is what makes the cost payable: it asserts they agree cell-for-cell
// over a seeded matrix that exercises every rung of the cascade.
//
// If you change either resolver and only one, this test fails. That is its job.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestResolveRecipientPreferencesAgreesWithGating(t *testing.T) {
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

	const extID = "pref-resolve-user"

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	err = pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'pref-resolve-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, 'Pref Resolve', $2, now(), now())
	`, extID, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	seedProjectPref := func(channel, topic, event, medium, name string, enabled bool) {
		_, err := pool.Exec(ctx, `
			INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, name, enabled, created_at, updated_at)
			VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, now(), now())
		`, projectID, channel, topic, event, medium, name, enabled)
		if err != nil {
			t.Fatalf("insert project preference: %v", err)
		}
	}

	seedRecipientPref := func(channel, topic, event, medium string, enabled bool) {
		_, err := pool.Exec(ctx, `
			INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, name, enabled, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, now(), now())
		`, projectID, extID, channel, topic, event, medium, enabled)
		if err != nil {
			t.Fatalf("insert recipient preference: %v", err)
		}
	}

	// The matrix. Each block exercises a distinct rung of the cascade, and the
	// three marked (*) are the cases the OLD catalog-merge read got wrong.
	//
	// A. Plain cataloged target, no recipient opinion.
	seedProjectPref("digest", "none", "sent", "in_app", "Daily digest", true)
	seedProjectPref("digest", "none", "sent", "email", "Daily digest", true)

	// B. Cataloged, recipient opted OUT of email (the unsubscribe shape).
	seedProjectPref("alerts", "none", "fired", "in_app", "Alerts", true)
	seedProjectPref("alerts", "none", "fired", "email", "Alerts", true)
	seedRecipientPref("alerts", "none", "fired", "email", false)

	// C. (*) UNCATALOGED target the recipient has an explicit email row for.
	//    Invisible to a catalog-only read, and it DELIVERS.
	seedRecipientPref("secret", "none", "ping", "email", true)

	// D. (*) Recipient topic='any' rule overriding an exact catalog row.
	seedProjectPref("posts", "post_1", "new_comment", "email", "Comments", false)
	seedRecipientPref("posts", "any", "new_comment", "email", true)

	// E. (*) Project topic='any' rule covering an UNCATALOGED exact target.
	//    cataloged=false, yet it delivers.
	seedProjectPref("news", "any", "digest", "email", "News", true)
	seedRecipientPref("news", "item_1", "digest", "in_app", true)

	// F. topic='none' must NEVER take the 'any' fallback.
	seedProjectPref("announce", "any", "new_feature", "email", "Announcements", true)
	seedRecipientPref("announce", "none", "new_feature", "in_app", true)

	repo := pg.NewPreferenceRepo(pool)

	resolved, err := repo.ResolveRecipientPreferences(ctx, projectID, extID, enum.ActiveMediums())
	if err != nil {
		t.Fatalf("ResolveRecipientPreferences: %v", err)
	}

	if len(resolved) == 0 {
		t.Fatal("resolver returned no cells")
	}

	// THE INVARIANT: every cell the grid renders must equal what a send would do.
	t.Run("every cell agrees with ShouldDirectNotificationBeDelivered", func(t *testing.T) {
		for _, cell := range resolved {
			target := dto.Target{Channel: cell.Channel, Topic: cell.Topic, Event: cell.Event}

			want, err := repo.ShouldDirectNotificationBeDelivered(ctx, projectID, extID, target, enum.Medium(cell.Medium))
			if err != nil {
				t.Fatalf("ShouldDirectNotificationBeDelivered(%s/%s/%s, %s): %v",
					cell.Channel, cell.Topic, cell.Event, cell.Medium, err)
			}

			if cell.Enabled != want {
				t.Errorf("cell %s/%s/%s medium=%s: grid says enabled=%v, send path says %v (source=%s, cataloged=%v)",
					cell.Channel, cell.Topic, cell.Event, cell.Medium,
					cell.Enabled, want, cell.Source, cell.Cataloged)
			}
		}
	})

	// Spot-check the cascade attribution and the measured table from the plan.
	// Agreement alone can't catch a resolver that is wrong in the SAME way as
	// the gating query, so pin the values that were actually measured.
	find := func(channel, topic, event, medium string) *struct {
		Enabled   bool
		Cataloged bool
		Source    string
	} {
		for _, c := range resolved {
			if c.Channel == channel && c.Topic == topic && c.Event == event && c.Medium == medium {
				return &struct {
					Enabled   bool
					Cataloged bool
					Source    string
				}{c.Enabled, c.Cataloged, string(c.Source)}
			}
		}
		return nil
	}

	cases := []struct {
		name                          string
		channel, topic, event, medium string
		wantEnabled, wantCataloged    bool
		wantSource                    string
	}{
		{
			name:    "cataloged in_app, no recipient row: inherits the catalog",
			channel: "digest", topic: "none", event: "sent", medium: "in_app",
			wantEnabled: true, wantCataloged: true, wantSource: "project_exact",
		},
		{
			name:    "cataloged email, recipient opted out: does not deliver",
			channel: "alerts", topic: "none", event: "fired", medium: "email",
			wantEnabled: false, wantCataloged: true, wantSource: "recipient_exact",
		},
		{
			// The measured table, row 3: an explicit recipient row on an
			// UNCATALOGED pair wins the cascade before the catalog is consulted.
			name:    "uncataloged email with recipient row enabled: DELIVERS and is visible",
			channel: "secret", topic: "none", event: "ping", medium: "email",
			wantEnabled: true, wantCataloged: false, wantSource: "recipient_exact",
		},
		{
			// The measured table, row 1: in_app's default is DELIVER, with no
			// catalog row anywhere. "Uncataloged ⇒ unavailable" is a lie here.
			name:    "uncataloged in_app, no row anywhere: delivers by default",
			channel: "secret", topic: "none", event: "ping", medium: "in_app",
			wantEnabled: true, wantCataloged: false, wantSource: "default",
		},
		{
			name:    "recipient topic=any rule beats an exact catalog row",
			channel: "posts", topic: "post_1", event: "new_comment", medium: "email",
			wantEnabled: true, wantCataloged: true, wantSource: "recipient_any",
		},
		{
			name:    "project topic=any rule covers an uncataloged exact target",
			channel: "news", topic: "item_1", event: "digest", medium: "email",
			wantEnabled: true, wantCataloged: false, wantSource: "project_any",
		},
		{
			// topic='none' means "this rule has no topic" — an 'any' rule must
			// not reach it, so email falls all the way to its default: OFF.
			name:    "topic=none never takes the any fallback",
			channel: "announce", topic: "none", event: "new_feature", medium: "email",
			wantEnabled: false, wantCataloged: false, wantSource: "default",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := find(tc.channel, tc.topic, tc.event, tc.medium)
			if got == nil {
				t.Fatalf("cell %s/%s/%s medium=%s missing from the resolved set",
					tc.channel, tc.topic, tc.event, tc.medium)
			}
			if got.Enabled != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", got.Enabled, tc.wantEnabled)
			}
			if got.Cataloged != tc.wantCataloged {
				t.Errorf("cataloged = %v, want %v", got.Cataloged, tc.wantCataloged)
			}
			if got.Source != tc.wantSource {
				t.Errorf("source = %q, want %q", got.Source, tc.wantSource)
			}
		})
	}

	t.Run("only active mediums are resolved", func(t *testing.T) {
		for _, c := range resolved {
			if !enum.Medium(c.Medium).Active() {
				t.Errorf("resolved a non-active medium %q — a toggle for a transport that cannot fire", c.Medium)
			}
		}
	})

	t.Run("another project's recipient rows never leak in", func(t *testing.T) {
		// external_id is unique only WITHIN a project, so the same id in another
		// project must not contribute cells (the 9.2 cross-project bug shape).
		var otherProject int
		err := pool.QueryRow(ctx, `
			INSERT INTO project (user_id, name, created_at, updated_at)
			VALUES ($1, 'pref-resolve-other', now(), now()) RETURNING id
		`, userID).Scan(&otherProject)
		if err != nil {
			t.Fatalf("insert other project: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", otherProject) })

		_, err = pool.Exec(ctx, `
			INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, name, enabled, created_at, updated_at)
			VALUES ($1, $2, 'leaked', 'none', 'evt', 'email', NULL, true, now(), now())
		`, otherProject, extID)
		if err != nil {
			t.Fatalf("insert other-project preference: %v", err)
		}

		again, err := repo.ResolveRecipientPreferences(ctx, projectID, extID, enum.ActiveMediums())
		if err != nil {
			t.Fatalf("ResolveRecipientPreferences: %v", err)
		}
		for _, c := range again {
			if c.Channel == "leaked" {
				t.Error("a preference from another project appeared in this recipient's grid")
			}
		}
	})

	// The Developer API's read used to be a Go exact-match merge that disagreed
	// with delivery, and a characterization test here pinned that divergence so
	// nobody re-pointed the console at it. The divergence is GONE — the read now
	// shares this resolver — so that test is replaced by its inverse: the
	// agreement it used to deny.
	t.Run("the Dev API read agrees with gating", func(t *testing.T) {
		recipientRepo := pg.NewRecipientRepo(pool)
		svc := NewProjectPreferenceService(repo, recipientRepo)

		got, _, err := svc.GetRecipientProjectPreferences(ctx, projectID, extID)
		if err != nil {
			t.Fatalf("GetRecipientProjectPreferences: %v", err)
		}

		findCell := func(channel, topic, event, medium string) *dto.PreferenceTargetResolvedStateDTO {
			for _, p := range got.Preferences {
				if p.Target.Channel == channel && p.Target.Topic == topic &&
					p.Target.Event == event && p.Target.Medium == medium {
					return p
				}
			}
			return nil
		}

		// THE INVARIANT, now on the public surface too: every cell it returns
		// must equal what a send would do.
		for _, p := range got.Preferences {
			target := dto.Target{Channel: p.Target.Channel, Topic: p.Target.Topic, Event: p.Target.Event}

			want, err := repo.ShouldDirectNotificationBeDelivered(ctx, projectID, extID, target, enum.Medium(p.Target.Medium))
			if err != nil {
				t.Fatalf("ShouldDirectNotificationBeDelivered: %v", err)
			}
			if p.State.Enabled != want {
				t.Errorf("cell %s/%s/%s medium=%s: Dev API read says enabled=%v, send path says %v",
					p.Target.Channel, p.Target.Topic, p.Target.Event, p.Target.Medium, p.State.Enabled, want)
			}
		}

		// The two cells the OLD read could not express at all. A recipient's row
		// on an uncataloged target DELIVERS, and in_app delivers by default —
		// both were invisible while being actively true.
		if c := findCell("secret", "none", "ping", "email"); c == nil {
			t.Error("uncataloged recipient row is missing — the row set regressed to the catalog-only merge")
		} else if !c.State.Enabled || c.State.Cataloged {
			t.Errorf("uncataloged recipient email cell: enabled=%v cataloged=%v, want true/false", c.State.Enabled, c.State.Cataloged)
		}

		if c := findCell("secret", "none", "ping", "in_app"); c == nil {
			t.Error("in_app default cell is missing — the read still ignores the medium-dependent default")
		} else if !c.State.Enabled {
			t.Error("in_app default cell should deliver")
		}
	})

	// The check endpoint answers ONE arbitrary (target, medium) — including
	// targets nothing is stored about — so it resolves an explicit universe
	// rather than filtering the read above. Same cascade, same answers.
	t.Run("the Dev API check agrees with gating", func(t *testing.T) {
		recipientRepo := pg.NewRecipientRepo(pool)
		svc := NewProjectPreferenceService(repo, recipientRepo)

		check := func(channel, topic, event, medium string) *dto.PreferenceTargetResolvedStateDTO {
			t.Helper()
			out, _, err := svc.CheckRecipientTargetSubscription(ctx, projectID, extID, dto.CheckRecipientTargetPayload{
				Target: dto.Target{Channel: channel, Topic: topic, Event: event},
				Medium: medium,
			})
			if err != nil {
				t.Fatalf("CheckRecipientTargetSubscription(%s/%s/%s, %s): %v", channel, topic, event, medium, err)
			}
			return out
		}

		// Every known cell: check must equal the send path.
		for _, cell := range resolved {
			want, err := repo.ShouldDirectNotificationBeDelivered(ctx, projectID, extID,
				dto.Target{Channel: cell.Channel, Topic: cell.Topic, Event: cell.Event}, enum.Medium(cell.Medium))
			if err != nil {
				t.Fatalf("ShouldDirectNotificationBeDelivered: %v", err)
			}

			if got := check(cell.Channel, cell.Topic, cell.Event, cell.Medium); got.State.Enabled != want {
				t.Errorf("check %s/%s/%s medium=%s: says enabled=%v, send path says %v",
					cell.Channel, cell.Topic, cell.Event, cell.Medium, got.State.Enabled, want)
			}
		}

		// A target stored NOWHERE — absent from the read's universe, yet it still
		// resolves. The old code defaulted every medium to enabled=true here, so
		// an email that would never fire reported as subscribed.
		if got := check("brand", "none", "new", "email"); got.State.Enabled {
			t.Error("unknown (target, email) reports subscribed, but email does not fire by default")
		}
		if got := check("brand", "none", "new", "in_app"); !got.State.Enabled {
			t.Error("unknown (target, in_app) reports unsubscribed, but in_app delivers by default")
		}

		// topic='none' must not take an 'any' rule: 'announce/any/new_feature'
		// email is cataloged true, but a 'none' topic can never reach it. The old
		// exact-match walk fell through to its default and said true.
		if got := check("announce", "none", "new_feature", "email"); got.State.Enabled {
			t.Error("topic=none took the 'any' fallback — it must not")
		}

		// A recipient 'any' rule the old exact-match walk could not see: it
		// returned the project's exact row (false) while the send path delivers.
		if got := check("posts", "post_1", "new_comment", "email"); !got.State.Enabled {
			t.Error("recipient topic=any rule ignored — it beats the exact catalog row")
		}
	})

	// Guard the reason the LEFT JOINs are safe: the partial unique indexes mean
	// each cascade rung matches at most one row, so no cell can fan out into
	// duplicates. If an index is ever dropped, this catches it.
	t.Run("no duplicate cells", func(t *testing.T) {
		seen := map[string]bool{}
		for _, c := range resolved {
			key := fmt.Sprintf("%s/%s/%s/%s", c.Channel, c.Topic, c.Event, c.Medium)
			if seen[key] {
				t.Errorf("duplicate cell %s — a cascade join fanned out", key)
			}
			seen[key] = true
		}
	})
}
