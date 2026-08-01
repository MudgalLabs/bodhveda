package pg

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// gateFixture spins up a scratch project against a live Postgres and returns the
// pool, the project id, and the preference repo under test. Self-cleaning.
func gateFixture(t *testing.T) (context.Context, *pgxpool.Pool, int, *PreferenceRepo) {
	t.Helper()

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
		VALUES ($1, 'catalog-gate-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	return ctx, pool, projectID, NewPreferenceRepo(pool).(*PreferenceRepo)
}

func catalogEntry(t *testing.T, pool *pgxpool.Pool, projectID int, channel, topic, event string, medium enum.Medium, enabled, mandatory bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, name, enabled, mandatory, created_at, updated_at)
		VALUES ($1, NULL, $2, $3, $4, $5, 'test entry', $6, $7, now(), now())
	`, projectID, channel, topic, event, string(medium), enabled, mandatory)
	if err != nil {
		t.Fatalf("insert catalog entry %s/%s/%s: %v", channel, topic, event, err)
	}
}

// TestCatalogGateMatchesWildcardTopic is the test that stands between strict
// targets and a broken Grahak.
//
// Grahak sends one target PER CONVERSATION — conversation/<conversationId>/reply,
// an unbounded set of runtime-generated topics — and catalogs exactly ONE
// wildcard row for all of them. Measured on production 2026-08-01, 8 of its reply
// sends match the catalog only via that wildcard. If the gate required an exact
// match, every one of them would 400, which is the product's core function.
func TestCatalogGateMatchesWildcardTopic(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	// Exactly what Grahak's seeder writes: one wildcard row, no per-thread rows.
	catalogEntry(t, pool, projectID, "conversation", "any", "reply", enum.MediumInApp, true, false)

	// What Grahak actually sends: a concrete, runtime-generated conversation id.
	sent := dto.Target{Channel: "conversation", Topic: "cms3a6gr6001yz83zpk8tp56y", Event: "reply"}

	cataloged, _, err := repo.LookupCatalogEntry(ctx, projectID, sent, enum.MediumInApp)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !cataloged {
		t.Fatal("a topic='any' catalog row must match a concrete topic; the gate would 400 every Grahak conversation reply")
	}

	// The wildcard must not be a skeleton key: a different event on the same
	// channel is still uncataloged.
	other := dto.Target{Channel: "conversation", Topic: "cms3a6gr6001yz83zpk8tp56y", Event: "deleted"}
	cataloged, _, err = repo.LookupCatalogEntry(ctx, projectID, other, enum.MediumInApp)
	if err != nil {
		t.Fatalf("lookup other event: %v", err)
	}
	if cataloged {
		t.Fatal("topic='any' widened the event too; the wildcard must only span topics")
	}

	// Nor may it cross mediums. Grahak catalogs in_app and email separately.
	cataloged, _, err = repo.LookupCatalogEntry(ctx, projectID, sent, enum.MediumEmail)
	if err != nil {
		t.Fatalf("lookup email: %v", err)
	}
	if cataloged {
		t.Fatal("an in_app catalog row must not satisfy the gate for the email medium")
	}
}

// TestCatalogGateDoesNotWidenTopicNone pins the one case the wildcard must NOT
// cover. 'none' is the reserved topic meaning "this rule has no topic" — widening
// it with a topic='any' rule would silently merge two distinct targets. The
// direct-send cascade has always carried this guard; the gate must agree, or the
// gate and the delivery decision disagree about what is cataloged.
func TestCatalogGateDoesNotWidenTopicNone(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "marketing", "any", "welcome", enum.MediumInApp, true, false)

	noneTopic := dto.Target{Channel: "marketing", Topic: "none", Event: "welcome"}

	cataloged, _, err := repo.LookupCatalogEntry(ctx, projectID, noneTopic, enum.MediumInApp)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if cataloged {
		t.Fatal("topic='any' must not match topic='none'; the cascade excludes it and the gate must too")
	}
}

// TestCatalogGateRejectsUncatalogedTarget is the base case — and the one that
// proves the gate is not simply always-true. The transposed target below is not
// invented for the test: conversation/reply/customer sat in production with its
// topic and event swapped, two notifications nothing could deliver and nobody
// could mute.
func TestCatalogGateRejectsUncatalogedTarget(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "conversation", "any", "reply", enum.MediumInApp, true, false)

	transposed := dto.Target{Channel: "conversation", Topic: "reply", Event: "customer"}

	cataloged, _, err := repo.LookupCatalogEntry(ctx, projectID, transposed, enum.MediumInApp)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if cataloged {
		t.Fatal("a transposed target must not be cataloged; catching this is the point of the gate")
	}
}

// TestCatalogGateIgnoresEnabledFlag pins §3.3: the gate is EXISTENCE-based.
//
// A cataloged-but-disabled entry is a real, deliberate state — "defined,
// currently switched off project-wide" — not a caller error. Sending to it must
// be accepted and simply reach nobody. If the gate consulted `enabled`, turning a
// target off project-wide would start returning 400s to a caller that did
// nothing wrong.
func TestCatalogGateIgnoresEnabledFlag(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "digest", "none", "sent", enum.MediumEmail, false, false)

	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}

	cataloged, _, err := repo.LookupCatalogEntry(ctx, projectID, target, enum.MediumEmail)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !cataloged {
		t.Fatal("a disabled catalog entry is still cataloged; the gate must not conflate 'switched off' with 'undefined'")
	}
}

// TestMandatoryEntryOverridesRecipientOptOut is what lets Arthveda's welcome
// message survive the gate.
//
// marketing/none/welcome is Arthveda's largest production target (796 sends) and
// is deliberately uncataloged today, because a toggle for a notification that
// fires exactly once is noise. Cataloging it as MANDATORY keeps that judgement
// intact: the gate passes, and the recipient's opt-out is ignored rather than
// silently honoured.
func TestMandatoryEntryOverridesRecipientOptOut(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "marketing", "none", "welcome", enum.MediumInApp, true, true)

	const recipient = "user-1"
	_, err := pool.Exec(ctx, `
		INSERT INTO recipient (project_id, external_id, created_at, updated_at)
		VALUES ($1, $2, now(), now())
	`, projectID, recipient)
	if err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	// The recipient explicitly opts out. A mandatory entry must ignore this.
	_, err = pool.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, enabled, created_at, updated_at)
		VALUES ($1, $2, 'marketing', 'none', 'welcome', 'in_app', false, now(), now())
	`, projectID, recipient)
	if err != nil {
		t.Fatalf("insert recipient opt-out: %v", err)
	}

	target := dto.Target{Channel: "marketing", Topic: "none", Event: "welcome"}

	cataloged, mandatory, err := repo.LookupCatalogEntry(ctx, projectID, target, enum.MediumInApp)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !cataloged || !mandatory {
		t.Fatalf("expected a mandatory catalog entry, got cataloged=%v mandatory=%v", cataloged, mandatory)
	}

	deliver, err := repo.ShouldDirectNotificationBeDelivered(ctx, projectID, recipient, target, enum.MediumInApp)
	if err != nil {
		t.Fatalf("should deliver: %v", err)
	}
	if !deliver {
		t.Fatal("a mandatory catalog entry must outrank the recipient's own opt-out")
	}

	// The console has to render such a cell as locked rather than as a toggle
	// that would silently do nothing, so the resolver must report it.
	resolved, err := repo.ResolveRecipientPreferenceForTargets(
		ctx, projectID, recipient, []enum.Medium{enum.MediumInApp}, []dto.Target{target},
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved cell, got %d", len(resolved))
	}
	if !resolved[0].Mandatory {
		t.Fatal("resolver must surface Mandatory so the UI can lock the toggle")
	}
	if !resolved[0].Enabled {
		t.Fatal("resolver disagrees with ShouldDirectNotificationBeDelivered about a mandatory entry")
	}
}

// TestMandatoryOffIsStillTheProjectsKillSwitch pins the limit of `mandatory`: it
// removes the RECIPIENT's choice, not the project's. A mandatory entry with
// enabled=false must not deliver — otherwise a project has no way to stop a
// notification it has decided to stop sending, short of deleting the entry and
// tripping the gate.
func TestMandatoryOffIsStillTheProjectsKillSwitch(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "marketing", "none", "welcome", enum.MediumInApp, false, true)

	const recipient = "user-1"
	if _, err := pool.Exec(ctx, `
		INSERT INTO recipient (project_id, external_id, created_at, updated_at)
		VALUES ($1, $2, now(), now())
	`, projectID, recipient); err != nil {
		t.Fatalf("insert recipient: %v", err)
	}

	// Even a recipient who explicitly opted IN must not receive it.
	if _, err := pool.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, enabled, created_at, updated_at)
		VALUES ($1, $2, 'marketing', 'none', 'welcome', 'in_app', true, now(), now())
	`, projectID, recipient); err != nil {
		t.Fatalf("insert recipient opt-in: %v", err)
	}

	target := dto.Target{Channel: "marketing", Topic: "none", Event: "welcome"}

	deliver, err := repo.ShouldDirectNotificationBeDelivered(ctx, projectID, recipient, target, enum.MediumInApp)
	if err != nil {
		t.Fatalf("should deliver: %v", err)
	}
	if deliver {
		t.Fatal("a disabled mandatory entry must not deliver; the project keeps its kill switch")
	}
}

// TestBroadcastEligibilityHonoursWildcardTopic closes a real inconsistency that
// predates strict targets: the three broadcast audience queries matched the topic
// EXACTLY while the direct-send cascade honoured a topic='any' catalog row. The
// same target therefore resolved one way for a direct send and the opposite way
// for a broadcast. The gate makes it reachable — a wildcard target now passes the
// gate and would then have found an audience of nobody.
func TestBroadcastEligibilityHonoursWildcardTopic(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "conversation", "any", "reply", enum.MediumInApp, true, false)

	for _, extID := range []string{"r1", "r2"} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO recipient (project_id, external_id, created_at, updated_at)
			VALUES ($1, $2, now(), now())
		`, projectID, extID); err != nil {
			t.Fatalf("insert recipient %s: %v", extID, err)
		}
	}

	// r2 mutes this one thread — the GitHub-style per-thread mute the wildcard
	// catalog exists to enable.
	if _, err := pool.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, enabled, created_at, updated_at)
		VALUES ($1, 'r2', 'conversation', 'thread-7', 'reply', 'in_app', false, now(), now())
	`, projectID); err != nil {
		t.Fatalf("insert per-thread mute: %v", err)
	}

	target := dto.Target{Channel: "conversation", Topic: "thread-7", Event: "reply"}

	eligible, err := repo.ListEligibleRecipientExtIDsForBroadcast(ctx, projectID, target, enum.MediumInApp)
	if err != nil {
		t.Fatalf("list eligible: %v", err)
	}
	if len(eligible) != 1 || eligible[0] != "r1" {
		t.Fatalf("expected exactly [r1] eligible via the wildcard catalog row, got %v", eligible)
	}

	// The aggregate the console tree renders must agree with the list, cell for cell.
	audience, err := repo.CountBroadcastAudience(ctx, projectID, target, enum.MediumInApp)
	if err != nil {
		t.Fatalf("count audience: %v", err)
	}
	if audience.Total != 2 || audience.Eligible != 1 || audience.ExcludedDisabled != 1 {
		t.Fatalf("audience disagrees with the eligible list: %+v", audience)
	}

	// And so must the per-batch filter the broadcast worker uses.
	filtered, err := repo.FilterEligibleRecipientsForBroadcast(ctx, projectID, target, enum.MediumInApp, []string{"r1", "r2"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != "r1" {
		t.Fatalf("per-batch filter disagrees with the eligible list, got %v", filtered)
	}
}

// TestUpdatingACatalogEntryDoesNotClearMandatory pins a regression that is
// invisible when it happens.
//
// `mandatory` was briefly a plain bool on the update payload. Every client that
// predates it — including the console's own edit modal — sends a body with no
// `mandatory` key, which decodes to false. Renaming a "Password reset" entry
// would therefore have quietly made it opt-out-able, with a 200 and no
// indication anything else had changed.
func TestUpdatingACatalogEntryDoesNotClearMandatory(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "security", "none", "password_reset", enum.MediumEmail, true, true)

	var id int
	if err := pool.QueryRow(ctx, `
		SELECT id FROM preference
		WHERE project_id = $1 AND recipient_external_id IS NULL AND event = 'password_reset'
	`, projectID).Scan(&id); err != nil {
		t.Fatalf("find catalog entry: %v", err)
	}

	// A caller changing only the name — no mention of mandatory.
	updated, err := repo.UpdateProjectPreference(ctx, projectID, id, "Password reset email", nil, true, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name == nil || *updated.Name != "Password reset email" {
		t.Fatalf("the rename did not apply: %+v", updated)
	}
	if !updated.Mandatory {
		t.Fatal("renaming an entry cleared its mandatory flag; omitting the field must leave it unchanged")
	}

	// And it must still be settable when the caller DOES mean it.
	off := false
	updated, err = repo.UpdateProjectPreference(ctx, projectID, id, "Password reset email", nil, true, &off)
	if err != nil {
		t.Fatalf("update clearing mandatory: %v", err)
	}
	if updated.Mandatory {
		t.Fatal("an explicit mandatory=false was ignored")
	}
}
