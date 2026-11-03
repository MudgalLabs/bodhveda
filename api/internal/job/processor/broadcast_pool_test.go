package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/dbx"
)

// TestBroadcastPipelineNeedsOnlyOneConnection pins the fix for broadcast 4
// (2026-11-03), which took 30m45s to deliver 857 notifications that each batch
// wrote in ~50ms.
//
// The worker runs 10 tasks at once on a pool of max(4, NumCPU) = 4 connections.
// Every broadcast task opens a transaction — pinning one connection — and some
// reads inside it went to the POOL, asking for a second. Nine concurrent
// batches took all four connections and each waited for another that only a
// fellow waiter could release, until Asynq's 30-minute timeout killed them all
// and the staggered retries finished in milliseconds. The same shape hit billing
// on 2026-07-28.
//
// A pool capped at ONE connection turns any such read into a self-deadlock, so
// this fails deterministically — no concurrency, no flakiness — the moment a
// pool read reappears anywhere inside the prepare or delivery transaction,
// including the email fan-out and billing reads it calls into.
func TestBroadcastPipelineNeedsOnlyOneConnection(t *testing.T) {
	seedPool := testPool(t) // seeding and assertions only, never handed to the code under test
	pool := singleConnPool(t)

	projectID := testProject(t, seedPool, "bcast-one-conn-test")

	// Email on, so the delivery transaction runs FanOutBroadcastEmail and every
	// read in it. No recipient has a contact, so no send task is produced and
	// nothing needs a real Asynq client: each lands as `no_contact`, which still
	// goes through eligibility, settings, contacts and the catalog lookup.
	recipients := []string{"r1", "r2", "r3"}
	seedEmailProject(t, seedPool, projectID, 100, recipients, nil)

	var userID int
	if err := seedPool.QueryRow(context.Background(),
		`SELECT user_id FROM project WHERE id = $1`, projectID).Scan(&userID); err != nil {
		t.Fatalf("read project user: %v", err)
	}

	// Long enough that a healthy run never trips it, short enough that the
	// deadlock reads as a failure rather than a hung suite.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	preferenceRepo := pg.NewPreferenceRepo(pool)
	broadcastRepo := pg.NewBroadcastRepo(pool)
	batchRepo := pg.NewBroadcastBatchRepo(pool)
	billing := service.NewBillingService(pool, pg.NewProjectRepo(pool), pg.NewUserSubscriptionRepo(pool),
		pg.NewUsageLogRepo(pool), pg.NewUsageAggregateRepo(pool))
	notifications := emailTestService(pool)

	broadcast, err := broadcastRepo.Create(ctx,
		entity.NewBroadcast(projectID, []byte(`{"t":"hi"}`), "product", "updates", "released").
			WithEmail("Subject", "<p>hi</p>", "hi"),
	)
	if err != nil {
		t.Fatalf("create broadcast: %v", err)
	}

	// prepareTx directly rather than ProcessTask: ProcessTask enqueues the
	// batches on Asynq after commit, which is outside the transaction and not
	// what this test is about.
	prepare := NewPrepareBroadcastBatchesProcessor(pool, nil, preferenceRepo, broadcastRepo, batchRepo, billing, notifications)
	target := dto.Target{Channel: broadcast.Channel, Topic: broadcast.Topic, Event: broadcast.Event}

	var batches []*entity.BroadcastBatch
	if err := dbx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return prepare.prepareTx(ctx, tx, broadcast, target, userID, &batches)
	}); err != nil {
		failOnDeadlock(t, "prepare", err)
	}
	if len(batches) == 0 {
		t.Fatal("prepare produced no batches; the delivery half of this test would prove nothing")
	}

	deliver := NewBroadcastDeliveryProcessor(pool, pg.NewNotificationRepo(pool), broadcastRepo, batchRepo,
		preferenceRepo, notifications, nil)

	for _, batch := range batches {
		payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
			ProjectID:       projectID,
			BroadcastID:     broadcast.ID,
			BatchID:         batch.ID,
			RecipientExtIDs: batch.RecipientExtIDs,
			Payload:         broadcast.Payload,
			Channel:         broadcast.Channel,
			Topic:           broadcast.Topic,
			Event:           broadcast.Event,
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}

		if err := deliver.ProcessTask(ctx, asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)); err != nil {
			failOnDeadlock(t, fmt.Sprintf("deliver batch %d", batch.ID), err)
		}
	}

	var delivered, noContact int
	if err := seedPool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM notification WHERE broadcast_id = $1 AND status = 'delivered'),
			(SELECT COUNT(*) FROM notification_delivery WHERE project_id = $2 AND status = 'no_contact')
	`, broadcast.ID, projectID).Scan(&delivered, &noContact); err != nil {
		t.Fatalf("count results: %v", err)
	}
	if delivered != len(recipients) {
		t.Errorf("delivered notifications = %d, want %d", delivered, len(recipients))
	}
	if noContact != len(recipients) {
		t.Errorf("no_contact email deliveries = %d, want %d — the email fan-out did not run", noContact, len(recipients))
	}
}

// singleConnPool opens a pool that can hold exactly one connection, so a pool
// read made while a transaction is open blocks on itself.
func singleConnPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DB_URL"))
	if err != nil {
		t.Fatalf("parse TEST_DB_URL: %v", err)
	}
	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func failOnDeadlock(t *testing.T, step string, err error) {
	t.Helper()
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%s: hung until the deadline on a 1-connection pool — something inside the "+
			"transaction read from the pool instead of the tx: %v", step, err)
	}
	t.Fatalf("%s: %v", step, err)
}
