package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// TestBroadcastAudienceMatchesEligibleList is the load-bearing test for the
// delivery tree's audience node.
//
// CountBroadcastAudience and ListEligibleRecipientExtIDsForBroadcast duplicate
// the same eligibility expression in two different shapes (an aggregate and a
// row list). If they ever drift, the console tree shows an eligible count that
// disagrees with the notifications actually written — a number that looks
// authoritative and is wrong. This pins them to each other against a live
// Postgres, across every combination of recipient-level and project-level
// preference that can exist.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestBroadcastAudienceMatchesEligibleList(t *testing.T) {
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
		VALUES ($1, 'audience-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}
	medium := enum.MediumInApp

	addRecipient := func(extID string) {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO recipient (project_id, external_id, created_at, updated_at)
			VALUES ($1, $2, now(), now())
		`, projectID, extID)
		if err != nil {
			t.Fatalf("insert recipient %s: %v", extID, err)
		}
	}

	prefRepo := NewPreferenceRepo(pool)

	setRecipientPref := func(extID string, enabled bool) {
		t.Helper()
		p := entity.NewPreference(&projectID, &extID, target.Channel, target.Topic, target.Event,
			string(medium), nil, nil, enabled)
		if _, err := prefRepo.Create(ctx, p); err != nil {
			t.Fatalf("create recipient pref for %s: %v", extID, err)
		}
	}

	setCatalogPref := func(enabled bool) {
		t.Helper()
		name := "Digest"
		p := entity.NewPreference(&projectID, nil, target.Channel, target.Topic, target.Event,
			string(medium), &name, nil, enabled)
		if _, err := prefRepo.Create(ctx, p); err != nil {
			t.Fatalf("create catalog pref: %v", err)
		}
	}

	// Every shape a recipient can be in, per the eligibility rule:
	//   recipient pref true                       -> eligible
	//   recipient pref false                      -> excluded_disabled
	//   no recipient pref, catalog enabled        -> eligible
	//   no recipient pref, catalog absent/disabled-> excluded_not_cataloged
	addRecipient("opted-in")
	setRecipientPref("opted-in", true)

	addRecipient("opted-out")
	setRecipientPref("opted-out", false)

	addRecipient("default-a")
	addRecipient("default-b")

	assertConsistent := func(label string, wantEligible, wantDisabled, wantNotCataloged int) {
		t.Helper()

		audience, err := prefRepo.CountBroadcastAudience(ctx, projectID, target, medium)
		if err != nil {
			t.Fatalf("%s: count audience: %v", label, err)
		}

		list, err := prefRepo.ListEligibleRecipientExtIDsForBroadcast(ctx, projectID, target, medium)
		if err != nil {
			t.Fatalf("%s: list eligible: %v", label, err)
		}

		// The one that matters: the two queries must agree.
		if audience.Eligible != len(list) {
			t.Errorf("%s: aggregate says %d eligible, list returned %d — the two eligibility predicates have drifted",
				label, audience.Eligible, len(list))
		}

		if audience.Eligible != wantEligible {
			t.Errorf("%s: eligible = %d, want %d", label, audience.Eligible, wantEligible)
		}
		if audience.ExcludedDisabled != wantDisabled {
			t.Errorf("%s: excluded_disabled = %d, want %d", label, audience.ExcludedDisabled, wantDisabled)
		}
		if audience.ExcludedNotCataloged != wantNotCataloged {
			t.Errorf("%s: excluded_not_cataloged = %d, want %d", label, audience.ExcludedNotCataloged, wantNotCataloged)
		}

		// The four buckets must partition the recipients exactly, or the tree
		// renders numbers that do not add up to the total it displays.
		if sum := audience.Eligible + audience.ExcludedDisabled + audience.ExcludedNotCataloged; sum != audience.Total {
			t.Errorf("%s: eligible(%d) + disabled(%d) + not_cataloged(%d) = %d, but total = %d",
				label, audience.Eligible, audience.ExcludedDisabled, audience.ExcludedNotCataloged,
				sum, audience.Total)
		}
	}

	// No catalog row yet: the two defaulted recipients are excluded because the
	// PROJECT never offered this target — not because they opted out. This is the
	// case the tree has to name precisely, since it is the usual reason a
	// broadcast silently reaches nobody.
	assertConsistent("no catalog row", 1, 1, 2)

	setCatalogPref(true)
	assertConsistent("catalog enabled", 3, 1, 0)
}

// A catalog row that exists but is DISABLED must land in not_cataloged, not in
// disabled — `excluded_disabled` means the recipient opted out, and attributing
// a project-level choice to the recipient sends an operator hunting in the wrong
// place.
func TestBroadcastAudienceDisabledCatalogIsNotRecipientOptOut(t *testing.T) {
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
		VALUES ($1, 'audience-disabled-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}
	medium := enum.MediumInApp
	prefRepo := NewPreferenceRepo(pool)

	if _, err := pool.Exec(ctx, `
		INSERT INTO recipient (project_id, external_id, created_at, updated_at)
		VALUES ($1, 'defaulted', now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	name := "Digest"
	catalog := entity.NewPreference(&projectID, nil, target.Channel, target.Topic, target.Event,
		string(medium), &name, nil, false)
	if _, err := prefRepo.Create(ctx, catalog); err != nil {
		t.Fatalf("create disabled catalog pref: %v", err)
	}

	audience, err := prefRepo.CountBroadcastAudience(ctx, projectID, target, medium)
	if err != nil {
		t.Fatalf("count audience: %v", err)
	}

	if audience.ExcludedNotCataloged != 1 {
		t.Errorf("a disabled catalog row should exclude as not_cataloged, got %d", audience.ExcludedNotCataloged)
	}
	if audience.ExcludedDisabled != 0 {
		t.Errorf("excluded_disabled must mean the RECIPIENT opted out, got %d", audience.ExcludedDisabled)
	}
	if audience.Eligible != 0 {
		t.Errorf("eligible = %d, want 0", audience.Eligible)
	}
}
