package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
)

// These two tests pin the connection-acquisition shape of CheckAndConsumeUsage.
//
// They exist because of the 2026-07-28 delivery-queue stall. CheckAndConsumeUsage
// used to run its plan/usage lookups INSIDE dbx.WithTx. The transaction pins one
// pool connection, and every one of those lookups queries the pool, so each
// in-flight call needed TWO connections simultaneously. pgxpool defaults to
// max(4, NumCPU) — 4 on the 2-core VPS — against a worker Concurrency of 10, so a
// burst of concurrent direct sends parked every goroutine holding a transaction
// connection while waiting for a second one only another blocked goroutine could
// release. pgxpool.Acquire waits until the context dies, so tasks hung for the
// full 1800s Asynq timeout and 57 notifications archived without ever delivering.
//
// Nothing about the payload, the target, or the medium was involved, which is why
// the payload-nullable feed test added alongside the feature could not have caught
// it. The axis that matters here is CONNECTIONS PER CALL, so that is what these
// assert.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.

// newBillingServiceForTest builds a real BillingService over a pool capped at
// maxConns, plus a throwaway project to meter against. Returns the service, the
// borrowed user id and the temp project id.
func newBillingServiceForTest(t *testing.T, maxConns int32) (*BillingService, int, int) {
	t.Helper()

	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	ctx := context.Background()

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	// The cap is the whole point of the test — set it explicitly rather than
	// inheriting pgxpool's NumCPU-derived default, so the test behaves the same
	// on a 2-core VPS and a 16-core laptop.
	cfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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
		VALUES ($1, 'billing-pool-test', now(), now()) RETURNING id
	`, userID).Scan(&projectID)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		// Children first — do not rely on cascade being configured.
		_, _ = pool.Exec(ctx, "DELETE FROM usage_log WHERE project_id = $1", projectID)
		_, _ = pool.Exec(ctx, "DELETE FROM usage_aggregate WHERE project_id = $1", projectID)
		_, _ = pool.Exec(ctx, "DELETE FROM project WHERE id = $1", projectID)
	})

	svc := NewBillingService(
		pool,
		pg.NewProjectRepo(pool),
		pg.NewUserSubscriptionRepo(pool),
		pg.NewUsageLogRepo(pool),
		pg.NewUsageAggregateRepo(pool),
	)

	return svc, userID, projectID
}

// completed reports whether err is a real outcome rather than the call having
// been killed by the deadline. ErrQuotaExceeded counts as completed: the test
// borrows a live user whose plan may genuinely be exhausted, and the regression
// under test is a HANG, not a particular billing verdict.
func completed(err error) bool {
	return err == nil || errors.Is(err, enum.ErrQuotaExceeded)
}

// TestCheckAndConsumeUsageNeedsOnlyOneConnection is the sharp one: a pool of
// exactly ONE connection and a single, entirely sequential call.
//
// No concurrency, so there is nothing racy about it — it either holds one
// connection at a time or it deadlocks against itself, deterministically. Before
// the fix this blocks forever on the nested acquire inside the transaction and
// fails on the deadline every single run.
func TestCheckAndConsumeUsageNeedsOnlyOneConnection(t *testing.T) {
	svc, userID, projectID := newBillingServiceForTest(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- svc.CheckAndConsumeUsage(ctx, dto.UsageEvent{
			UserID:    userID,
			ProjectID: projectID,
			Metric:    entity.MetricNotifications,
			Amount:    1,
		})
	}()

	select {
	case err := <-done:
		if !completed(err) {
			t.Fatalf("CheckAndConsumeUsage failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("CheckAndConsumeUsage deadlocked on a single-connection pool: it is " +
			"holding a transaction connection while acquiring another. Keep the plan/usage " +
			"lookups outside dbx.WithTx.")
	}
}

// TestCheckAndConsumeUsageConcurrentBurstDoesNotDeadlock reproduces the
// production shape: more concurrent callers than the pool has connections, which
// is exactly what Grahak's 25-sends-in-5-seconds burst did to a worker running
// Concurrency: 10 against a 4-connection pool.
//
// Every call must finish. Before the fix all of them block until the deadline.
func TestCheckAndConsumeUsageConcurrentBurstDoesNotDeadlock(t *testing.T) {
	const (
		poolSize = 4  // pgxpool's default on the 2-core VPS: max(4, NumCPU)
		callers  = 10 // asynq.Config.Concurrency in internal/job/asynq.go
	)

	svc, userID, projectID := newBillingServiceForTest(t, poolSize)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, callers)

	for i := range callers {
		wg.Go(func() {
			errs[i] = svc.CheckAndConsumeUsage(ctx, dto.UsageEvent{
				UserID:    userID,
				ProjectID: projectID,
				Metric:    entity.MetricNotifications,
				Amount:    1,
			})
		})
	}

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatalf("%d concurrent CheckAndConsumeUsage calls deadlocked a %d-connection pool; "+
			"this is the 2026-07-28 delivery-queue stall", callers, poolSize)
	}

	for i, err := range errs {
		if !completed(err) {
			t.Errorf("caller %d failed: %v", i, err)
		}
	}
}
