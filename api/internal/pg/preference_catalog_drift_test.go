package pg

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// sentNotification writes a direct send with a target. `payload` present means
// the send asked for in_app, which is the same rule the drift query applies.
func sentNotification(t *testing.T, pool *pgxpool.Pool, projectID int, channel, topic, event string) int {
	t.Helper()

	var id int
	err := pool.QueryRow(context.Background(), `
		INSERT INTO notification (project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
		VALUES ($1, 'drift-user', '{"t":1}'::jsonb, $2, $3, $4, 'delivered', now(), now())
		RETURNING id
	`, projectID, channel, topic, event).Scan(&id)
	if err != nil {
		t.Fatalf("insert notification %s/%s/%s: %v", channel, topic, event, err)
	}
	return id
}

func driftFor(t *testing.T, targets []*entity.UncatalogedTarget, channel, medium string) *entity.UncatalogedTarget {
	t.Helper()
	for _, target := range targets {
		if target.Channel == channel && target.Medium == medium {
			return target
		}
	}
	return nil
}

// TestCatalogDriftReportsUncatalogedSends is the report's base case. The setting
// defaults to OFF, so nothing forces a project to notice that its code names
// targets its catalog does not define — this query is the only thing that does.
func TestCatalogDriftReportsUncatalogedSends(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "billing", "none", "invoice_paid", enum.MediumInApp, true, false)

	sentNotification(t, pool, projectID, "billing", "none", "invoice_paid") // cataloged
	sentNotification(t, pool, projectID, "prodcut", "none", "updated")      // the typo case
	sentNotification(t, pool, projectID, "prodcut", "none", "updated")

	targets, err := repo.ListUncatalogedSentTargets(ctx, projectID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if drift := driftFor(t, targets, "billing", "in_app"); drift != nil {
		t.Error("a cataloged target must not appear as drift; the report would cry wolf")
	}

	drift := driftFor(t, targets, "prodcut", "in_app")
	if drift == nil {
		t.Fatal("an uncataloged target that was actually sent must be reported")
	}
	if drift.Sends != 2 {
		t.Errorf("sends = %d, want 2", drift.Sends)
	}
}

// TestCatalogDriftAgreesWithTheGate is the important one. The report exists to be
// trusted enough that someone turns the gate on, so it must resolve the catalog
// EXACTLY as the gate does — including the wildcard, which is the rule most
// likely to be re-implemented slightly differently in a second query.
//
// Grahak's shape: one topic='any' entry covering an unbounded set of concrete
// topics. If the report matched exactly, it would list every conversation id as
// drift and tell its owner to catalog thousands of rows.
func TestCatalogDriftAgreesWithTheGate(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	catalogEntry(t, pool, projectID, "conversation", "any", "reply", enum.MediumInApp, true, false)

	sentNotification(t, pool, projectID, "conversation", "cms3a6gr6001yz83zpk8tp56y", "reply")
	sentNotification(t, pool, projectID, "conversation", "cms3a6gr6001yz83zpk8tp56z", "reply")

	targets, err := repo.ListUncatalogedSentTargets(ctx, projectID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if drift := driftFor(t, targets, "conversation", "in_app"); drift != nil {
		t.Fatalf("a wildcard-cataloged target was reported as drift (%s/%s/%s); the report and the gate disagree",
			drift.Channel, drift.Topic, drift.Event)
	}

	// And the agreement must be real in both directions, not vacuous: a target
	// the gate WOULD reject has to show up.
	sentNotification(t, pool, projectID, "conversation", "none", "reply")

	targets, err = repo.ListUncatalogedSentTargets(ctx, projectID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("list after none-topic send: %v", err)
	}

	drift := driftFor(t, targets, "conversation", "in_app")
	if drift == nil || drift.Topic != "none" {
		t.Fatal("topic='none' is never widened by a wildcard, so the gate rejects it and the report must list it")
	}
}

// TestCatalogDriftIgnoresUntargetedSends pins the exclusion. An untargeted send
// stores empty channel/topic/event and is never gated, whatever the setting — so
// listing it would tell someone to catalog a target that does not exist.
func TestCatalogDriftIgnoresUntargetedSends(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	sentNotification(t, pool, projectID, "", "", "")

	targets, err := repo.ListUncatalogedSentTargets(ctx, projectID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, target := range targets {
		if target.Channel == "" {
			t.Fatal("an untargeted send was reported as an uncataloged target")
		}
	}
}

// TestCatalogDriftRespectsTheWindow keeps the report from nagging about a target
// the project stopped sending long ago.
func TestCatalogDriftRespectsTheWindow(t *testing.T) {
	ctx, pool, projectID, repo := gateFixture(t)

	id := sentNotification(t, pool, projectID, "legacy", "none", "thing")
	if _, err := pool.Exec(ctx, `UPDATE notification SET created_at = now() - interval '90 days' WHERE id = $1`, id); err != nil {
		t.Fatalf("age the notification: %v", err)
	}

	targets, err := repo.ListUncatalogedSentTargets(ctx, projectID, time.Now().UTC().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if drift := driftFor(t, targets, "legacy", "in_app"); drift != nil {
		t.Error("a send older than the window was reported")
	}
}
