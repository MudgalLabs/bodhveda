# Fan-out benchmarks

How long Bodhveda takes to fan one broadcast out to 10k / 100k / 1M recipients,
and how many in-app notifications per second that is.

Every number in this file was produced by running the code in this directory.
None of it is estimated or extrapolated. If you change the fan-out path, re-run
it and update the table — a perf claim nobody can reproduce is worse than no
claim.

## Results

**2026-08-02, commit `289f2d6`, hardware and caveats below.**

| Recipients | Batches | Prepare | Deliver | **Total** | **Throughput** | Batch p50 / p99 | Recipient p50 / p99 |
| ---------: | ------: | ------: | ------: | --------: | -------------: | --------------: | ------------------: |
|     10,000 |      10 |    15 ms |    93 ms | **108 ms** |  **93,021/s** |   90 / 92 ms |     104 / 107 ms |
|    100,000 |     100 |   110 ms |   581 ms | **691 ms** | **144,736/s** |  53 / 103 ms |     430 / 691 ms |
|  1,000,000 |   1,000 |  1.13 s |   7.43 s | **8.56 s** | **116,876/s** |  70 / 119 ms |    4.67 / 8.31 s |

Three runs per size (`-benchtime=3x`); the table is the mean of the runs, except
the percentiles, which are the last run's. Run-to-run spread was under 5%
(1M: 8.35 s / 8.50 s / 8.81 s).

What the two latency columns mean:

- **Batch p50/p99** — how long one batch of 1,000 recipients takes to deliver.
  This is the unit of work a worker picks off the queue.
- **Recipient p50/p99** — how long after the send starts a given *person's*
  notification is in their inbox. Weighted by batch size, so it is a
  distribution over recipients, not over batches. The p99 is essentially the
  total: the last batch finishes when the fan-out finishes.

### Where the time goes

| Stage | 10k | 100k | 1M |
| --- | ---: | ---: | ---: |
| Audience cascade — list eligible (`ListEligibleRecipientExtIDsForBroadcast`) | 7.2 ms | 26.1 ms | 273 ms |
| Audience cascade — count breakdown (`CountBroadcastAudience`) | 5.5 ms | 21.1 ms | 209 ms |
| Asynq enqueue, one task per batch | 5.3 ms | 24.5 ms | 239 ms |

Enqueue costs ~0.24 ms per batch task and is the one production step the
end-to-end timing above excludes (see "What is not measured"). Adding it back
puts 1M at ~8.8 s.

Delivery is ~87% of the wall clock at 1M and is where any further work belongs.
It is not one big insert: it is 1,000 transactions, each re-checking in-app
eligibility for its 1,000 recipients and then bulk-inserting 1,000 rows into
`notification`, which carries four indexes.

## ⚠️ No plan can actually send this today

The 100k and 1M rows above describe the pipeline, not something a customer can
do. `entity.GetPlan` caps notifications at **10,000/period (free)** and
**100,000/period (pro)**, and `PrepareBroadcastBatchesProcessor` consumes that
quota for the whole audience in one event — so a 1M-recipient broadcast is
refused at the quota check and never fans out at all.

That is why the harness omits the billing step (see `prepare()` in
`fanout_bench_test.go`): running it would benchmark the rejection path. The
omission costs nothing in accuracy — the quota check is one subscription read,
one aggregate read and one usage write, all O(1) per broadcast regardless of
audience size.

**So: quote the 100k number freely, and treat 1M as a capacity statement about
the engine, not a plan feature.** Selling "fan out to a million" while the
highest plan stops at 100k is the kind of gap the first engineer who tries it
finds immediately.

## Environment

The numbers above are from a single developer laptop, with the database in a
container on the same machine.

**These stand as the published figures for now, quoted as what they are: an
Apple M4 laptop.** Prod is a VPS with different disks, so the same run there will
land somewhere else — measuring it is a later task, not a blocker on using these.
Whatever gets quoted, quote the hardware with it.

| | |
| --- | --- |
| Host | Apple M4, 10 cores, 16 GB RAM, macOS 26.6 (25G72) |
| Go | 1.25.6 darwin/arm64 |
| Postgres | 17.4 (Alpine, aarch64) in Docker Desktop, 10 CPUs / 7.75 GB to the VM |
| Postgres config | stock: `shared_buffers=128MB`, `work_mem=4MB`, `fsync=on`, `synchronous_commit=on`, `max_wal_size=1GB` |
| Delivery concurrency | 10, matching `asynq.Config{Concurrency: 10}` in `cmd/worker` |
| Audience shape | every recipient eligible via one project-level catalog entry; no recipient-level rows |

## What is measured

One `BenchmarkFanout` iteration is a whole broadcast, using the real
repositories and the real `BroadcastDeliveryProcessor` the worker runs:

1. Insert the `broadcast` row.
2. **Prepare**, in one transaction: lock the broadcast, run the eligibility
   cascade to list the audience, count the audience breakdown, freeze it onto
   the broadcast, and write one `broadcast_batch` row per 1,000 recipients.
3. **Deliver**: every batch through `BroadcastDeliveryProcessor.ProcessTask`,
   across 10 goroutines. Each task re-derives in-app eligibility for its batch,
   bulk-inserts the `notification` rows, marks the batch `success`, and the last
   one completes the broadcast.
4. **Assert**: exactly N rows landed with status `delivered` and the broadcast
   reached `completed`. A run that delivers fewer rows fails instead of
   reporting a flattering rate.

Between iterations (timer stopped) the broadcast is deleted — cascading to its
notifications and batches — and `notification` is vacuumed, so run *i+1* is not
inserting into a table full of run *i*'s dead tuples.

## What is not measured

- **Asynq/Redis transit.** Delivery tasks are executed in-process rather than
  round-tripped through the queue. `BenchmarkFanoutEnqueue` measures the enqueue
  half against a real Redis (0.24 ms/task); dequeue and scheduling overhead are
  not measured, and neither is queue wait time on a worker that is also doing
  something else.
- **The billing quota check** — see above.
- **Broadcast email.** These broadcasts are in-app only, which is also all a
  broadcast can be today (email is direct-only), so the email branch returns
  immediately — exactly as it does in production for a broadcast with no email
  half.
- **API request handling.** The send endpoint's own work (auth, validation,
  enqueueing `broadcast:prepare_batches`) is a few milliseconds and independent
  of audience size.
- **Network.** App and database are on the same host.

## Running them

They are opt-in on `TEST_DB_URL`, like the DB integration tests, and skip
entirely without it. **Point them at a scratch database, not at anything you
care about** — a 1M run seeds a million recipients and writes a million
notifications, then deletes them.

```bash
cd api

# Everything, on the local dev database.
TEST_DB_URL="postgres://postgres:postgres@localhost:42070/bodhveda?sslmode=disable" \
  go test ./internal/bench/ -run '^$' -bench BenchmarkFanout -benchtime=3x -timeout=60m -v

# One size, quickly.
TEST_DB_URL=... BENCH_SIZES=10000 \
  go test ./internal/bench/ -run '^$' -bench BenchmarkFanout -benchtime=1x -v

# Stage breakdown.
TEST_DB_URL=... go test ./internal/bench/ -run '^$' -bench BenchmarkAudienceQuery -benchtime=5x

# Enqueue cost. BENCH_REDIS_URL must name a Redis DB of its own (here: /9) —
# never the dev queue, or a running worker will pick up benchmark tasks.
TEST_DB_URL=... BENCH_REDIS_URL="redis://:password@localhost:6776/9" \
  go test ./internal/bench/ -run '^$' -bench BenchmarkFanoutEnqueue -benchtime=3x
```

`-v` prints a per-run line with the full breakdown; without it you get Go's
benchmark line, which carries the same figures as custom metrics
(`notifs/s`, `prepare_ms`, `deliver_ms`, `batch_p50_ms`, `recip_p99_ms`, …).

Seeding 1M recipients takes ~5.5 s and happens once per `go test` invocation,
shared by every benchmark in that run; the fixture project is dropped in
`TestMain`. A run that is killed mid-flight leaves a `bench-fanout-<n>` project
behind — `DELETE FROM project WHERE name LIKE 'bench-fanout%'` cleans it up.

### Knobs

| Env | Default | |
| --- | --- | --- |
| `TEST_DB_URL` | — | required; benchmarks skip without it |
| `BENCH_SIZES` | `10000,100000,1000000` | audience sizes to run |
| `BENCH_CONCURRENCY` | `10` | delivery goroutines; also sizes the pool |
| `BENCH_OPTOUT_PCT` | `0` | share of recipients given a `disabled` preference row of their own, so the cascade has to exclude them |
| `BENCH_REDIS_URL` | — | `BenchmarkFanoutEnqueue` skips without it |
