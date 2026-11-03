package app

import (
	"net/url"
	"strconv"

	jobs "github.com/mudgallabs/bodhveda/internal/job"
)

// DBPoolMaxConns is the pgxpool ceiling for the api and worker processes.
//
// ⚠️ It is a deadlock floor, not a throughput knob. pgxpool defaults to
// max(4, NumCPU), which is 4 on the 2-core prod VPS, while the worker runs
// jobs.Concurrency (10) tasks at once. A task that holds a transaction and
// then reads from the pool needs TWO connections; enough of those at once take
// every connection and each waits on another that only a fellow waiter could
// release. That hung the delivery queue for the full 30-minute Asynq timeout on
// 2026-07-28 (billing) and again on 2026-11-03 (broadcast 4).
//
// Both were fixed at the source (reads inside a transaction go through the tx,
// pinned by TestBroadcastPipelineNeedsOnlyOneConnection), but nothing stops the
// next one being written. With two connections per concurrent task the pool
// cannot run dry from this shape, so a future slip costs a connection instead
// of a half-hour outage. api + worker = 40 against Postgres' default 100.
var DBPoolMaxConns = 2 * jobs.Concurrency

// withPoolMaxConns sets pool_max_conns on a postgres URL unless it already sets
// one, in which case the operator's explicit value wins.
//
// ⚠️ Applied here, in code, and never in BODHVEDA_DB_URL itself: goose shares
// that variable in the migrate step and rejects pgxpool-only parameters, which
// silently aborted a deploy once. A DSN that is not a URL is returned as-is.
func withPoolMaxConns(dbURL string, maxConns int) string {
	u, err := url.Parse(dbURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return dbURL
	}

	q := u.Query()
	if q.Has("pool_max_conns") {
		return dbURL
	}
	q.Set("pool_max_conns", strconv.Itoa(maxConns))
	u.RawQuery = q.Encode()

	return u.String()
}
