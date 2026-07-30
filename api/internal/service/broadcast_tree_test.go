package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestBroadcastDeliveryTree exercises the console's broadcast tree end to end
// against a real Postgres: the frozen audience breakdown, the per-status rollup,
// and the outcome folding.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestBroadcastDeliveryTree(t *testing.T) {
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
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, 'tree-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})

	broadcastRepo := pg.NewBroadcastRepo(pool)
	notificationRepo := pg.NewNotificationRepo(pool)
	svc := NewBroadcastService(broadcastRepo, notificationRepo)

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "digest", "none", "sent"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	seed := func(extID string, status enum.NotificationStatus) {
		t.Helper()
		_, err := pool.Exec(ctx, `
			INSERT INTO notification (project_id, recipient_external_id, payload, broadcast_id,
				channel, topic, event, created_at, updated_at, status)
			VALUES ($1, $2, '{}'::jsonb, $3, 'digest', 'none', 'sent', now(), now(), $4)
		`, projectID, extID, broadcast.ID, string(status))
		if err != nil {
			t.Fatalf("seed notification %s: %v", extID, err)
		}
	}

	seed("r1", enum.NotificationStatusDelivered)
	seed("r2", enum.NotificationStatusDelivered)
	seed("r3", enum.NotificationStatusMuted)
	seed("r4", enum.NotificationStatusFailed)
	seed("r5", enum.NotificationStatusEnqueued)

	t.Run("audience is nil before fan-out records it", func(t *testing.T) {
		// A broadcast whose audience was never measured must render as "not
		// recorded", never as a zero — a broadcast that legitimately reached
		// nobody is a different fact from one nobody measured.
		tree, errKind, err := svc.GetDeliveryTree(ctx, projectID, broadcast.ID)
		if err != nil {
			t.Fatalf("get tree: %v (%v)", err, errKind)
		}
		if tree.Audience != nil {
			t.Errorf("audience should be nil until prepare_batches records it, got %+v", tree.Audience)
		}
	})

	if err := broadcastRepo.SetAudience(ctx, broadcast.ID, &entity.BroadcastAudience{
		Total:                10,
		Eligible:             5,
		ExcludedDisabled:     2,
		ExcludedNotCataloged: 3,
	}); err != nil {
		t.Fatalf("set audience: %v", err)
	}

	tree, errKind, err := svc.GetDeliveryTree(ctx, projectID, broadcast.ID)
	if err != nil {
		t.Fatalf("get tree: %v (%v)", err, errKind)
	}

	if tree.Kind != enum.NotificationKindBroadcast {
		t.Errorf("kind = %q, want broadcast", tree.Kind)
	}
	if tree.Target.Channel != "digest" {
		t.Errorf("target channel = %q, want digest", tree.Target.Channel)
	}

	// --- Audience ------------------------------------------------------------
	if tree.Audience == nil {
		t.Fatal("audience should be present once recorded")
	}
	if tree.Audience.Total != 10 || tree.Audience.Eligible != 5 {
		t.Errorf("audience = %+v, want total 10 / eligible 5", tree.Audience)
	}
	// The two exclusions are different problems and must stay separate: one is a
	// recipient opting out, the other is the project never offering the target.
	if tree.Audience.ExcludedDisabled != 2 || tree.Audience.ExcludedNotCataloged != 3 {
		t.Errorf("exclusions = %+v, want disabled 2 / not_cataloged 3", tree.Audience)
	}
	// ⚠️ Never expandable for a broadcast — excluded recipients leave no rows, so
	// the console must not offer a drill-down that cannot exist.
	if tree.Audience.Expandable {
		t.Error("broadcast audience exclusions must not be expandable — there are no rows to open")
	}

	// --- Mediums -------------------------------------------------------------
	if len(tree.Mediums) != 1 {
		t.Fatalf("want exactly one medium branch (broadcasts are in-app only), got %d", len(tree.Mediums))
	}

	m := tree.Mediums[0]
	if m.Medium != string(enum.MediumInApp) {
		t.Errorf("medium = %q, want in_app", m.Medium)
	}
	if m.Total != 5 {
		t.Errorf("total = %d, want 5", m.Total)
	}
	if got := m.Outcomes[string(enum.OutcomeSucceeded)]; got != 2 {
		t.Errorf("succeeded = %d, want 2", got)
	}
	if got := m.Outcomes[string(enum.OutcomeSuppressed)]; got != 1 {
		t.Errorf("suppressed = %d, want 1 (muted is not a failure)", got)
	}
	if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 1 {
		t.Errorf("failed = %d, want 1", got)
	}
	// The stalled-send signal: an enqueued row long after the send means the
	// worker never resolved it.
	if m.Pending != 1 {
		t.Errorf("pending = %d, want 1", m.Pending)
	}
}

// A broadcast id belonging to another project must 404 rather than return an
// empty tree. The rollup query is keyed by broadcast_id alone (so it stays
// covered by ix_notification_broadcast), which means without this check a
// cross-project id would render as a legitimately empty broadcast — leaking its
// existence and lying about its contents.
func TestBroadcastDeliveryTreeRejectsCrossProjectAccess(t *testing.T) {
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

	newProject := func(name string) int {
		t.Helper()
		var id int
		if err := pool.QueryRow(ctx, `
			INSERT INTO project (user_id, name, created_at, updated_at)
			VALUES ($1, $2, now(), now()) RETURNING id
		`, userID, name).Scan(&id); err != nil {
			t.Fatalf("insert project %s: %v", name, err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, id)
		})
		return id
	}

	owner := newProject("tree-owner")
	intruder := newProject("tree-intruder")

	broadcastRepo := pg.NewBroadcastRepo(pool)
	svc := NewBroadcastService(broadcastRepo, pg.NewNotificationRepo(pool))

	broadcast, err := broadcastRepo.Create(ctx, entity.NewBroadcast(owner, []byte(`{}`), "digest", "none", "sent"))
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	tree, errKind, err := svc.GetDeliveryTree(ctx, intruder, broadcast.ID)
	if err == nil {
		t.Fatal("reading another project's broadcast must fail, not return a tree")
	}
	if tree != nil {
		t.Error("no tree should be returned for a cross-project broadcast")
	}
	if errKind != tantraService.ErrNotFound {
		t.Errorf("errKind = %v, want ErrNotFound", errKind)
	}
}
