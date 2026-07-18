package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

// TestUpsertProjectPreferences exercises the declarative bulk merge against a
// live Postgres: insert-new + update-existing by natural key, merge leaving
// absent rows untouched, prune removing them, and — the load-bearing one — prune
// never touching a recipient's own rows.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestUpsertProjectPreferences(t *testing.T) {
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

	extID := "upsert-user"

	var userID int
	if err := pool.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	err = pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'upsert-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, 'Upsert', $2, now(), now())
	`, extID, projectID)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	repo := NewPreferenceRepo(pool)

	// A recipient-level row on the SAME target/medium as a catalog entry below —
	// prune must never remove it.
	if _, err := repo.Create(ctx, entity.NewPreference(&projectID, &extID, "alerts", "none", "fired", "email", nil, false)); err != nil {
		t.Fatalf("seed recipient row: %v", err)
	}

	lbl := func(s string) *string { return &s }

	// find is a small helper: locate a (channel,event,medium) row in a catalog.
	find := func(catalog []*entity.Preference, channel, event, medium string) *entity.Preference {
		for _, p := range catalog {
			if p.Channel == channel && p.Event == event && p.Medium == medium {
				return p
			}
		}
		return nil
	}

	// 1. Initial merge — two new catalog entries.
	set1 := []*entity.Preference{
		{ProjectID: &projectID, Channel: "digest", Topic: "none", Event: "sent", Medium: "email", Label: lbl("Digest"), Enabled: true},
		{ProjectID: &projectID, Channel: "alerts", Topic: "none", Event: "fired", Medium: "email", Label: lbl("Alerts"), Enabled: true},
	}
	cat, err := repo.UpsertProjectPreferences(ctx, projectID, set1, false)
	if err != nil {
		t.Fatalf("upsert set1: %v", err)
	}
	if len(cat) != 2 {
		t.Fatalf("after set1 want 2 catalog rows, got %d", len(cat))
	}
	if d := find(cat, "digest", "sent", "email"); d == nil || d.Label == nil || *d.Label != "Digest" || !d.Enabled {
		t.Fatalf("digest not inserted correctly: %+v", d)
	}

	// 2. Merge again: UPDATE the digest label/default, ADD a new entry, and OMIT
	//    alerts. Without prune, alerts must survive untouched.
	digestID := find(cat, "digest", "sent", "email").ID
	set2 := []*entity.Preference{
		{ProjectID: &projectID, Channel: "digest", Topic: "none", Event: "sent", Medium: "email", Label: lbl("Weekly Digest"), Enabled: false},
		{ProjectID: &projectID, Channel: "news", Topic: "any", Event: "posted", Medium: "email", Label: lbl("News"), Enabled: true},
	}
	cat, err = repo.UpsertProjectPreferences(ctx, projectID, set2, false)
	if err != nil {
		t.Fatalf("upsert set2 (merge): %v", err)
	}
	if len(cat) != 3 {
		t.Fatalf("after merge want 3 catalog rows (digest, alerts, news), got %d", len(cat))
	}
	d := find(cat, "digest", "sent", "email")
	if d == nil || d.ID != digestID {
		t.Fatalf("digest should be updated in place, not re-created: %+v", d)
	}
	if d.Label == nil || *d.Label != "Weekly Digest" || d.Enabled {
		t.Fatalf("digest update did not apply: %+v", d)
	}
	if find(cat, "alerts", "fired", "email") == nil {
		t.Fatal("merge removed the omitted 'alerts' row; it must be left untouched")
	}

	// 3. Prune: the set is digest + news only, so alerts (absent) is removed.
	set3 := []*entity.Preference{
		{ProjectID: &projectID, Channel: "digest", Topic: "none", Event: "sent", Medium: "email", Label: lbl("Weekly Digest"), Enabled: false},
		{ProjectID: &projectID, Channel: "news", Topic: "any", Event: "posted", Medium: "email", Label: lbl("News"), Enabled: true},
	}
	cat, err = repo.UpsertProjectPreferences(ctx, projectID, set3, true)
	if err != nil {
		t.Fatalf("upsert set3 (prune): %v", err)
	}
	if len(cat) != 2 {
		t.Fatalf("after prune want 2 catalog rows, got %d", len(cat))
	}
	if find(cat, "alerts", "fired", "email") != nil {
		t.Fatal("prune should have removed the catalog 'alerts' row")
	}

	// 4. The recipient's OWN alerts/email row must have survived the prune.
	var recipientRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM preference
		WHERE project_id = $1 AND recipient_external_id = $2
		  AND channel = 'alerts' AND topic = 'none' AND event = 'fired' AND medium = 'email'
	`, projectID, extID).Scan(&recipientRows); err != nil {
		t.Fatalf("count recipient rows: %v", err)
	}
	if recipientRows != 1 {
		t.Fatalf("prune touched a recipient row: want 1 surviving, got %d", recipientRows)
	}
}
