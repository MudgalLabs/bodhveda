package bench

// Broadcast fan-out throughput, measured end to end against a live Postgres.
//
// The question this answers is "how long does it take, and how many
// notifications per second, to fan one broadcast out to N recipients" — for N of
// 10k, 100k and 1M. It is the number quoted on the marketing page, so it is
// measured, never estimated: the harness runs the SAME repositories and the SAME
// job processor the worker runs, and asserts afterwards that exactly N
// notification rows landed. A run that delivers fewer rows fails rather than
// reporting a flattering rate.
//
// What is and is not in the measured window is documented in README.md. The two
// deliberate omissions, both stated there: Asynq/Redis transit between prepare
// and delivery (BenchmarkFanoutEnqueue measures the enqueue half separately),
// and the billing quota check, which no plan would let a 1M-recipient broadcast
// past today — see the note on prepare() below.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/job/processor"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/dbx"
)

// The broadcast every benchmark sends. A concrete topic (not "none") is
// deliberate: it makes all four rungs of the eligibility cascade join, which is
// the expensive shape of the audience query rather than the cheap one.
var (
	benchTarget  = dto.Target{Channel: "bench", Topic: "weekly", Event: "digest"}
	benchPayload = json.RawMessage(`{"title":"Your weekly digest","body":"3 new replies, 2 mentions","url":"https://example.com/inbox"}`)
)

// defaultConcurrency mirrors asynq.Config{Concurrency: 10} in cmd/worker, so the
// delivery half runs at the same parallelism a single deployed worker has.
const defaultConcurrency = 10

func benchSizes(b *testing.B) []int {
	raw := os.Getenv("BENCH_SIZES")
	if raw == "" {
		return []int{10_000, 100_000, 1_000_000}
	}

	var sizes []int
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			b.Fatalf("BENCH_SIZES: %q is not a number: %v", part, err)
		}
		sizes = append(sizes, n)
	}

	return sizes
}

func benchConcurrency(b *testing.B) int {
	raw := os.Getenv("BENCH_CONCURRENCY")
	if raw == "" {
		return defaultConcurrency
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		b.Fatalf("BENCH_CONCURRENCY: %q is not a positive number", raw)
	}

	return n
}

// optOutPct is the share of recipients given a recipient-level preference row of
// their own that says `enabled = false`. Zero by default so that N recipients
// means exactly N notifications and the headline number is literal; raise it to
// measure a project where part of the audience has muted the target.
func optOutPct(b *testing.B) int {
	raw := os.Getenv("BENCH_OPTOUT_PCT")
	if raw == "" {
		return 0
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 100 {
		b.Fatalf("BENCH_OPTOUT_PCT: %q is not a percentage", raw)
	}

	return n
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// fixture is a seeded project: N recipients and a catalog entry that opts them
// into benchTarget. Seeding 1M recipients is minutes of work, so fixtures are
// built once per size and shared by every benchmark in the run, then dropped in
// TestMain.
type fixture struct {
	pool      *pgxpool.Pool
	projectID int
	// eligible is how many of the N recipients the broadcast should reach — it is
	// what the harness asserts the fan-out actually wrote.
	eligible int
}

var (
	fixturesMu sync.Mutex
	fixtures   = map[int]*fixture{}
	benchPool  *pgxpool.Pool
)

func TestMain(m *testing.M) {
	code := m.Run()
	dropFixtures()
	if benchPool != nil {
		benchPool.Close()
	}
	os.Exit(code)
}

func dropFixtures() {
	fixturesMu.Lock()
	defer fixturesMu.Unlock()

	for _, fx := range fixtures {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		if _, err := fx.pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, fx.projectID); err != nil {
			fmt.Fprintf(os.Stderr, "bench: drop fixture project %d: %v\n", fx.projectID, err)
		}
		cancel()
	}
}

// pool opens the benchmark pool, skipping the whole run when TEST_DB_URL is
// unset — the same opt-in the DB integration tests use.
//
// ⚠️ MaxConns has to clear the delivery concurrency. pgxpool defaults to
// max(4, NumCPU), and a pool smaller than the number of delivering goroutines
// silently serialises them, which reads as the database being slow.
func pool(b *testing.B) *pgxpool.Pool {
	b.Helper()

	fixturesMu.Lock()
	defer fixturesMu.Unlock()

	if benchPool != nil {
		return benchPool
	}

	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		b.Skip("TEST_DB_URL not set; skipping fan-out benchmark")
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		b.Fatalf("parse TEST_DB_URL: %v", err)
	}
	cfg.MaxConns = int32(benchConcurrency(b) + 4)

	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		b.Fatalf("connect: %v", err)
	}

	benchPool = p

	return benchPool
}

func fixtureFor(b *testing.B, n int) *fixture {
	b.Helper()

	p := pool(b)

	fixturesMu.Lock()
	if fx, ok := fixtures[n]; ok {
		fixturesMu.Unlock()
		return fx
	}
	fixturesMu.Unlock()

	ctx := context.Background()
	start := time.Now()

	var userID int
	if err := p.QueryRow(ctx, `SELECT user_id FROM project ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		b.Fatalf("need at least one existing project to borrow a user_id: %v", err)
	}

	var projectID int
	if err := p.QueryRow(ctx, `
		INSERT INTO project (user_id, name, created_at, updated_at)
		VALUES ($1, $2, now(), now()) RETURNING id
	`, userID, fmt.Sprintf("bench-fanout-%d", n)).Scan(&projectID); err != nil {
		b.Fatalf("insert project: %v", err)
	}

	fx := &fixture{pool: p, projectID: projectID}

	// COPY, not INSERT: seeding is not what is being measured, and at 1M rows the
	// difference is minutes.
	now := time.Now().UTC()
	copied, err := p.CopyFrom(ctx,
		pgx.Identifier{"recipient"},
		[]string{"project_id", "external_id", "name", "created_at", "updated_at"},
		pgx.CopyFromSlice(n, func(i int) ([]any, error) {
			return []any{projectID, recipientExtID(i), "", now, now}, nil
		}),
	)
	if err != nil {
		b.Fatalf("copy recipients: %v", err)
	}
	if copied != int64(n) {
		b.Fatalf("copied %d recipients, want %d", copied, n)
	}

	// The project-level catalog entry. This is the `pp` rung of the cascade: with
	// it enabled and no recipient-level row, every recipient is eligible.
	if _, err := p.Exec(ctx, `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, name, enabled, medium, created_at, updated_at)
		VALUES ($1, NULL, $2, $3, $4, 'Bench weekly digest', true, 'in_app', now(), now())
	`, projectID, benchTarget.Channel, benchTarget.Topic, benchTarget.Event); err != nil {
		b.Fatalf("insert catalog preference: %v", err)
	}

	fx.eligible = n

	if pct := optOutPct(b); pct > 0 {
		optedOut := 0
		_, err := p.CopyFrom(ctx,
			pgx.Identifier{"preference"},
			[]string{"project_id", "recipient_external_id", "channel", "topic", "event", "enabled", "medium", "created_at", "updated_at"},
			pgx.CopyFromSlice(n*pct/100, func(i int) ([]any, error) {
				optedOut++
				// Spread the opt-outs across the id space rather than clustering them
				// in the first rows, so the audience query cannot get a run of
				// same-answer rows it would never see in production.
				return []any{
					projectID, recipientExtID(i * 100 / pct), benchTarget.Channel, benchTarget.Topic,
					benchTarget.Event, false, "in_app", now, now,
				}, nil
			}),
		)
		if err != nil {
			b.Fatalf("copy opt-out preferences: %v", err)
		}
		fx.eligible = n - optedOut
	}

	// ⚠️ Without ANALYZE the planner is working from empty-table statistics on a
	// table that just gained a million rows, and picks plans no production
	// database would pick. Autovacuum would get there eventually; the benchmark
	// cannot wait for eventually.
	if _, err := p.Exec(ctx, `ANALYZE recipient, preference`); err != nil {
		b.Fatalf("analyze: %v", err)
	}

	b.Logf("seeded fixture: %d recipients, %d eligible, project %d, in %s",
		n, fx.eligible, projectID, time.Since(start).Round(time.Millisecond))

	fixturesMu.Lock()
	fixtures[n] = fx
	fixturesMu.Unlock()

	return fx
}

func recipientExtID(i int) string {
	return fmt.Sprintf("bench-recipient-%08d", i)
}

// ---------------------------------------------------------------------------
// The pipeline under test
// ---------------------------------------------------------------------------

// deps are the real repositories and the real delivery processor, wired exactly
// as cmd/worker wires them.
type deps struct {
	pool       *pgxpool.Pool
	broadcast  repository.BroadcastRepository
	batch      repository.BroadcastBatchRepository
	preference repository.PreferenceRepository
	delivery   *processor.BroadcastDeliveryProcessor
}

func newDeps(b *testing.B) *deps {
	b.Helper()

	p := pool(b)

	broadcastRepo := pg.NewBroadcastRepo(p)
	batchRepo := pg.NewBroadcastBatchRepo(p)
	notificationRepo := pg.NewNotificationRepo(p)
	preferenceRepo := pg.NewPreferenceRepo(p)

	// ⚠️ A real NotificationService, not nil, so the delivery processor takes the
	// same branch the worker takes. It returns immediately for a broadcast with no
	// email half, which is what these benchmarks send — but skipping the call
	// would mean benchmarking a code path that does not ship. Its billing,
	// recipient-service and Asynq dependencies are nil because only the email
	// fan-out path reaches them, and that path is never entered here.
	notificationService := service.NewNotificationService(
		notificationRepo, pg.NewRecipientRepo(p), preferenceRepo, broadcastRepo, batchRepo,
		pg.NewNotificationDeliveryRepo(p), pg.NewRecipientContactRepo(p),
		pg.NewProjectEmailSettingsRepo(p), pg.NewProjectRepo(p),
		nil, nil, nil,
	)

	return &deps{
		pool:       p,
		broadcast:  broadcastRepo,
		batch:      batchRepo,
		preference: preferenceRepo,
		delivery: processor.NewBroadcastDeliveryProcessor(
			p, notificationRepo, broadcastRepo, batchRepo, preferenceRepo, notificationService, nil,
		),
	}
}

// batchSizeFor mirrors the slicing in PrepareBroadcastBatchesProcessor.prepareTx.
//
// ⚠️ Kept in step with it BY HAND — nothing enforces it. It is duplicated rather
// than called because prepareTx is unexported and, more importantly, because it
// consumes billing quota (see prepare() below). If the slicing there changes,
// this must follow, or the benchmark quietly stops describing the shipped
// pipeline: the numbers stay plausible while the batch count is wrong.
func batchSizeFor(n int) int {
	if n <= 100 {
		return n
	}
	return min(max(n/10, 100), 1000)
}

// prepare runs the preparation half of a fan-out: freeze the audience, and write
// one broadcast_batch row per chunk of recipients, in one transaction.
//
// ⚠️ One step of the shipped prepare is deliberately absent: the billing quota
// check. It is O(1) per broadcast (one subscription read, one usage-aggregate
// read, one usage-log write) and so contributes nothing to the shape of these
// curves — but it would REFUSE the 100k and 1M runs outright, because the plans
// in entity/plan.go cap notifications at 10k (free) and 100k (pro) per period.
// Running the shipped processor here would therefore benchmark the quota
// rejection path, not the fan-out. The ceiling is a product fact worth stating
// alongside the numbers, not a thing to measure around silently.
func (d *deps) prepare(ctx context.Context, fx *fixture) (*entity.Broadcast, []*entity.BroadcastBatch, error) {
	broadcast, err := d.broadcast.Create(ctx, entity.NewBroadcast(
		fx.projectID, benchPayload, benchTarget.Channel, benchTarget.Topic, benchTarget.Event,
	))
	if err != nil {
		return nil, nil, fmt.Errorf("create broadcast: %w", err)
	}

	var batches []*entity.BroadcastBatch

	err = dbx.WithTx(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := d.broadcast.StatusForUpdateTx(ctx, tx, broadcast.ID); err != nil {
			return fmt.Errorf("lock broadcast: %w", err)
		}

		extIDs, err := d.preference.ListEligibleRecipientExtIDsForBroadcast(ctx, fx.projectID, benchTarget, enum.MediumInApp)
		if err != nil {
			return fmt.Errorf("list eligible recipients: %w", err)
		}

		audience, err := d.preference.CountBroadcastAudience(ctx, fx.projectID, benchTarget, enum.MediumInApp)
		if err != nil {
			return fmt.Errorf("count audience: %w", err)
		}
		audience.Eligible = len(extIDs)

		if err := d.broadcast.SetAudienceTx(ctx, tx, broadcast.ID, audience); err != nil {
			return fmt.Errorf("set audience: %w", err)
		}

		size := batchSizeFor(len(extIDs))
		for i := 0; i < len(extIDs); i += size {
			end := min(i+size, len(extIDs))

			batch, err := d.batch.CreateTx(ctx, tx, entity.NewBroadcastBatch(broadcast.ID, extIDs[i:end]))
			if err != nil {
				return fmt.Errorf("create batch: %w", err)
			}

			batches = append(batches, batch)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return broadcast, batches, nil
}

// batchResult is one batch's contribution to the latency distributions: when it
// finished relative to the start of the fan-out, how long it itself took, and
// how many recipients that covers.
type batchResult struct {
	finishedAt time.Duration
	took       time.Duration
	recipients int
}

// deliver runs every batch through the real BroadcastDeliveryProcessor at the
// given concurrency, which is what the worker does with the tasks prepare
// enqueued.
func (d *deps) deliver(
	ctx context.Context, broadcast *entity.Broadcast, batches []*entity.BroadcastBatch, concurrency int, since time.Time,
) ([]batchResult, error) {
	work := make(chan *entity.BroadcastBatch, len(batches))
	for _, batch := range batches {
		work <- batch
	}
	close(work)

	results := make([]batchResult, len(batches))
	errs := make([]error, concurrency)

	var wg sync.WaitGroup
	var idx sync.Mutex
	next := 0

	for w := 0; w < concurrency; w++ {
		wg.Add(1)

		go func(worker int) {
			defer wg.Done()

			for batch := range work {
				payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
					ProjectID:       broadcast.ProjectID,
					BroadcastID:     broadcast.ID,
					BatchID:         batch.ID,
					RecipientExtIDs: batch.RecipientExtIDs,
					Payload:         broadcast.Payload,
					Channel:         broadcast.Channel,
					Topic:           broadcast.Topic,
					Event:           broadcast.Event,
				})
				if err != nil {
					errs[worker] = fmt.Errorf("marshal delivery payload: %w", err)
					return
				}

				started := time.Now()
				if err := d.delivery.ProcessTask(ctx, asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)); err != nil {
					errs[worker] = fmt.Errorf("deliver batch %d: %w", batch.ID, err)
					return
				}
				done := time.Now()

				idx.Lock()
				results[next] = batchResult{
					finishedAt: done.Sub(since),
					took:       done.Sub(started),
					recipients: batch.Recipients,
				}
				next++
				idx.Unlock()
			}
		}(w)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkFanout is the headline: one broadcast, N recipients, prepared and
// delivered, timed end to end.
func BenchmarkFanout(b *testing.B) {
	concurrency := benchConcurrency(b)

	for _, n := range benchSizes(b) {
		b.Run(fmt.Sprintf("recipients=%d", n), func(b *testing.B) {
			fx := fixtureFor(b, n)
			d := newDeps(b)
			ctx := context.Background()

			var (
				totalPrepare  time.Duration
				totalDeliver  time.Duration
				lastBatchP50  time.Duration
				lastBatchP99  time.Duration
				lastRecipP50  time.Duration
				lastRecipP99  time.Duration
				lastBatchSize int
			)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				broadcast, batches, err := d.prepare(ctx, fx)
				if err != nil {
					b.Fatalf("prepare: %v", err)
				}
				prepared := time.Now()

				results, err := d.deliver(ctx, broadcast, batches, concurrency, start)
				if err != nil {
					b.Fatalf("deliver: %v", err)
				}
				elapsed := time.Since(start)

				b.StopTimer()

				assertFanout(b, d.pool, broadcast.ID, fx.eligible)

				totalPrepare += prepared.Sub(start)
				totalDeliver += elapsed - prepared.Sub(start)
				lastBatchSize = len(batches)
				lastBatchP50, lastBatchP99 = batchServiceTimes(results)
				lastRecipP50, lastRecipP99 = recipientLatencies(results)

				b.Logf("n=%d run=%d: total %s (prepare %s, deliver %s), %d batches x %d workers, "+
					"%.0f notifications/s | batch service p50 %s p99 %s | recipient latency p50 %s p99 %s",
					n, i+1, elapsed.Round(time.Millisecond), prepared.Sub(start).Round(time.Millisecond),
					(elapsed - prepared.Sub(start)).Round(time.Millisecond), len(batches), concurrency,
					float64(fx.eligible)/elapsed.Seconds(),
					lastBatchP50.Round(time.Millisecond), lastBatchP99.Round(time.Millisecond),
					lastRecipP50.Round(time.Millisecond), lastRecipP99.Round(time.Millisecond))

				cleanupBroadcast(b, d.pool, broadcast.ID)

				b.StartTimer()
			}

			b.StopTimer()

			if b.N > 0 {
				runs := float64(b.N)
				perRun := time.Duration(float64(totalPrepare+totalDeliver) / runs)

				b.ReportMetric(float64(fx.eligible)/perRun.Seconds(), "notifs/s")
				b.ReportMetric(float64(totalPrepare)/runs/float64(time.Millisecond), "prepare_ms")
				b.ReportMetric(float64(totalDeliver)/runs/float64(time.Millisecond), "deliver_ms")
				b.ReportMetric(float64(lastBatchP50)/float64(time.Millisecond), "batch_p50_ms")
				b.ReportMetric(float64(lastBatchP99)/float64(time.Millisecond), "batch_p99_ms")
				b.ReportMetric(float64(lastRecipP50)/float64(time.Millisecond), "recip_p50_ms")
				b.ReportMetric(float64(lastRecipP99)/float64(time.Millisecond), "recip_p99_ms")
				b.ReportMetric(float64(lastBatchSize), "batches")
			}
		})
	}
}

// BenchmarkAudienceQuery isolates the size-dependent half of preparation: the
// cascade query that decides who the broadcast reaches.
func BenchmarkAudienceQuery(b *testing.B) {
	for _, n := range benchSizes(b) {
		fx := fixtureFor(b, n)
		d := newDeps(b)
		ctx := context.Background()

		b.Run(fmt.Sprintf("list/recipients=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				extIDs, err := d.preference.ListEligibleRecipientExtIDsForBroadcast(ctx, fx.projectID, benchTarget, enum.MediumInApp)
				if err != nil {
					b.Fatalf("list eligible: %v", err)
				}
				if len(extIDs) != fx.eligible {
					b.Fatalf("listed %d eligible recipients, want %d", len(extIDs), fx.eligible)
				}
			}
		})

		b.Run(fmt.Sprintf("count/recipients=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				audience, err := d.preference.CountBroadcastAudience(ctx, fx.projectID, benchTarget, enum.MediumInApp)
				if err != nil {
					b.Fatalf("count audience: %v", err)
				}
				if audience.Eligible != fx.eligible {
					b.Fatalf("counted %d eligible recipients, want %d", audience.Eligible, fx.eligible)
				}
			}
		})
	}
}

// BenchmarkFanoutEnqueue measures the piece BenchmarkFanout leaves out: pushing
// one delivery task per batch onto Asynq. Skipped unless BENCH_REDIS_URL is set,
// which MUST name a Redis database of its own (e.g. .../9) — the harness deletes
// every task it enqueues, but pointing it at the dev queue would still put
// benchmark tasks in front of a running worker.
func BenchmarkFanoutEnqueue(b *testing.B) {
	redisURL := os.Getenv("BENCH_REDIS_URL")
	if redisURL == "" {
		b.Skip("BENCH_REDIS_URL not set; skipping queue benchmark")
	}

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		b.Fatalf("parse BENCH_REDIS_URL: %v", err)
	}

	client := asynq.NewClient(opt)
	defer client.Close()

	inspector := asynq.NewInspector(opt)
	defer inspector.Close()

	// One task per batch of 1000, which is how many a broadcast of this size
	// produces.
	for _, n := range benchSizes(b) {
		batches := (n + batchSizeFor(n) - 1) / batchSizeFor(n)

		b.Run(fmt.Sprintf("batches=%d", batches), func(b *testing.B) {
			ctx := context.Background()
			extIDs := make([]string, batchSizeFor(n))
			for i := range extIDs {
				extIDs[i] = recipientExtID(i)
			}

			payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
				ProjectID: 0, BroadcastID: 0, BatchID: 0, RecipientExtIDs: extIDs,
				Payload: benchPayload, Channel: benchTarget.Channel, Topic: benchTarget.Topic, Event: benchTarget.Event,
			})
			if err != nil {
				b.Fatalf("marshal: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ids := make([]string, 0, batches)

				for j := 0; j < batches; j++ {
					id := fmt.Sprintf("bench-enqueue-%d-%d", i, j)
					if _, err := client.EnqueueContext(ctx,
						asynq.NewTask(task.TaskTypeBroadcastDelivery, payload),
						asynq.MaxRetry(3), asynq.TaskID(id),
					); err != nil {
						b.Fatalf("enqueue: %v", err)
					}
					ids = append(ids, id)
				}

				b.StopTimer()
				for _, id := range ids {
					if err := inspector.DeleteTask("default", id); err != nil {
						b.Fatalf("delete enqueued task: %v", err)
					}
				}
				b.StartTimer()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Assertions, cleanup, statistics
// ---------------------------------------------------------------------------

// assertFanout is what keeps these numbers honest: a run that wrote fewer rows
// than it claimed recipients, or left the broadcast unfinished, fails instead of
// reporting a throughput it did not achieve.
func assertFanout(b *testing.B, p *pgxpool.Pool, broadcastID, wantDelivered int) {
	b.Helper()

	ctx := context.Background()

	var rows, delivered int
	if err := p.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = $2)
		FROM notification WHERE broadcast_id = $1
	`, broadcastID, string(enum.NotificationStatusDelivered)).Scan(&rows, &delivered); err != nil {
		b.Fatalf("count notifications: %v", err)
	}

	if delivered != wantDelivered {
		b.Fatalf("broadcast %d delivered %d notifications (%d rows total), want %d",
			broadcastID, delivered, rows, wantDelivered)
	}

	var status string
	if err := p.QueryRow(ctx, `SELECT status FROM broadcast WHERE id = $1`, broadcastID).Scan(&status); err != nil {
		b.Fatalf("read broadcast status: %v", err)
	}
	if status != string(enum.BroadcastStatusCompleted) {
		b.Fatalf("broadcast %d ended in status %q, want %q", broadcastID, status, enum.BroadcastStatusCompleted)
	}
}

// cleanupBroadcast drops the run's notifications and batches (cascaded from the
// broadcast row) and reclaims the space, so run i+1 is not inserting into a table
// carrying run i's dead tuples.
func cleanupBroadcast(b *testing.B, p *pgxpool.Pool, broadcastID int) {
	b.Helper()

	ctx := context.Background()

	if _, err := p.Exec(ctx, `DELETE FROM broadcast WHERE id = $1`, broadcastID); err != nil {
		b.Fatalf("delete broadcast: %v", err)
	}
	if _, err := p.Exec(ctx, `VACUUM (ANALYZE) notification`); err != nil {
		b.Fatalf("vacuum notification: %v", err)
	}
}

// batchServiceTimes reports how long a single batch took to deliver — the unit
// of work the worker actually schedules.
func batchServiceTimes(results []batchResult) (p50, p99 time.Duration) {
	samples := make([]time.Duration, len(results))
	for i, r := range results {
		samples[i] = r.took
	}
	return percentile(samples, 0.50), percentile(samples, 0.99)
}

// recipientLatencies reports, per RECIPIENT, how long after the send began their
// notification became visible. A batch commits atomically, so every recipient in
// it shares that batch's finish time; weighting by batch size is what makes this
// a distribution over people rather than over batches.
func recipientLatencies(results []batchResult) (p50, p99 time.Duration) {
	total := 0
	for _, r := range results {
		total += r.recipients
	}

	sorted := make([]batchResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].finishedAt < sorted[j].finishedAt })

	at := func(q float64) time.Duration {
		target := int(math.Ceil(q * float64(total)))
		seen := 0
		for _, r := range sorted {
			seen += r.recipients
			if seen >= target {
				return r.finishedAt
			}
		}
		if len(sorted) == 0 {
			return 0
		}
		return sorted[len(sorted)-1].finishedAt
	}

	return at(0.50), at(0.99)
}

func percentile(samples []time.Duration, q float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(math.Ceil(q*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}
