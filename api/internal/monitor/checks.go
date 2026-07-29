package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// The queue the whole app enqueues to. Asynq is configured with no Queues map
// (internal/job/asynq.go), so every task type — notification delivery, email,
// broadcast fan-out, cascading deletes — shares this one queue.
const defaultQueue = "default"

// Thresholds. Deliberately loose: this monitor exists to catch an outage that
// went unnoticed for a day, not to page on jitter. Every one of these is
// tuned to fire on the two failure modes described in
// agent-docs/delivery-feedback-design.md §2.2 and stay quiet otherwise.
const (
	// queueLatencyThreshold is how old the OLDEST PENDING task may get before we
	// call the queue backed up. Normal latency is sub-second — anything minutes
	// old means tasks are arriving faster than they are consumed, or not being
	// consumed at all.
	queueLatencyThreshold = 5 * time.Minute

	// failureRatioThreshold is the share of today's processed tasks that may fail
	// before we alert. The incident that motivated this ran at 164/168 = 98%.
	failureRatioThreshold = 0.25
	// failureRatioMinProcessed stops a cold-start blip (1 failure out of 1) from
	// firing at 100%. Asynq's Processed/Failed are daily counters that reset, so
	// just after midnight the sample is legitimately tiny.
	failureRatioMinProcessed = 20

	// stuckSendAge is how long a notification may sit unresolved before it counts
	// as stuck. Comfortably beyond a normal send (sub-second) plus Asynq's retry
	// backoff for a transient blip.
	stuckSendAge = 10 * time.Minute
	// stuckSendWindow bounds the lookback so this query can never degrade into a
	// full-history scan as the table grows. Anything older is not actionable
	// anyway — it is history, not an incident.
	stuckSendWindow = 6 * time.Hour
)

// Check is one named health check.
type Check struct {
	Name string
	// MinConsecutive is how many unhealthy ticks in a row open the condition.
	// 1 = alert on the first bad reading (use for unambiguous signals);
	// 2 = require it to persist (use where a single tick could be a blip, e.g.
	// one long-running task briefly holding queue latency up).
	MinConsecutive int
	Run            func(ctx context.Context) (Finding, error)
}

// queueInspector is the slice of *asynq.Inspector this package needs. Narrowed
// to an interface so the checks are testable without a Redis.
type queueInspector interface {
	Servers() ([]*asynq.ServerInfo, error)
	GetQueueInfo(queue string) (*asynq.QueueInfo, error)
}

// stuckCounter is the slice of the notification repository this package needs.
type stuckCounter interface {
	CountStuck(ctx context.Context, olderThan, newerThan time.Time) (int, error)
}

// DefaultChecks builds the standard check set.
//
// ⚠️ The two failure modes below are DIFFERENT and need different checks. Do not
// trim this list to "is the worker alive?" — that is checkWorkerAbsent alone,
// and it would NOT have caught the incident this feature was built for:
//
//   - Worker crashed / never started → the queue has nobody consuming it.
//     Caught by checkWorkerAbsent + checkQueueLatency.
//   - Worker RUNNING but failing everything → it was consuming and archiving
//     tasks the whole time, so Servers() looked perfectly healthy.
//     Caught by checkFailureRatio + checkArchivedGrowth.
//
// checkStuckSends is the backstop for a third mode: the worker is alive and
// reporting success, yet notifications never reach a terminal state.
func DefaultChecks(inspector queueInspector, notifications stuckCounter, now func() time.Time) []Check {
	if now == nil {
		now = time.Now
	}

	archived := &archivedGrowthCheck{}

	return []Check{
		{
			Name:           "worker_absent",
			MinConsecutive: 2,
			Run:            func(ctx context.Context) (Finding, error) { return checkWorkerAbsent(inspector) },
		},
		{
			Name:           "queue_backed_up",
			MinConsecutive: 2,
			Run:            func(ctx context.Context) (Finding, error) { return checkQueueLatency(inspector) },
		},
		{
			Name:           "task_failure_ratio",
			MinConsecutive: 1,
			Run:            func(ctx context.Context) (Finding, error) { return checkFailureRatio(inspector) },
		},
		{
			Name:           "tasks_archived",
			MinConsecutive: 1,
			Run:            func(ctx context.Context) (Finding, error) { return archived.run(inspector) },
		},
		{
			Name:           "stuck_sends",
			MinConsecutive: 2,
			Run:            func(ctx context.Context) (Finding, error) { return checkStuckSends(ctx, notifications, now) },
		},
	}
}

// checkWorkerAbsent fires when no Asynq server is registered at all.
//
// Asynq servers heartbeat into Redis with a TTL, so a crashed or stopped worker
// disappears from Servers() on its own within ~a minute. MinConsecutive 2 covers
// the gap between that expiry and a restarting worker re-registering.
func checkWorkerAbsent(inspector queueInspector) (Finding, error) {
	servers, err := inspector.Servers()
	if err != nil {
		return Finding{}, fmt.Errorf("inspect asynq servers: %w", err)
	}

	if len(servers) > 0 {
		return Finding{}, nil
	}

	return Finding{
		Firing:  true,
		Summary: "No Asynq worker is registered — nothing is consuming the queue.",
		Fields: map[string]string{
			"queue": defaultQueue,
			"hint":  "check the worker container/process",
		},
	}, nil
}

// checkQueueLatency fires when the oldest pending task is older than
// queueLatencyThreshold. QueueInfo.Latency is exactly that age, and it is 0 on
// an empty queue — so a healthy idle system never trips this.
func checkQueueLatency(inspector queueInspector) (Finding, error) {
	info, err := inspector.GetQueueInfo(defaultQueue)
	if err != nil {
		return Finding{}, fmt.Errorf("inspect queue %q: %w", defaultQueue, err)
	}

	if info.Latency <= queueLatencyThreshold {
		return Finding{}, nil
	}

	return Finding{
		Firing: true,
		Summary: fmt.Sprintf("Queue is backed up — oldest pending task is %s old (threshold %s).",
			info.Latency.Round(time.Second), queueLatencyThreshold),
		Fields: map[string]string{
			"queue":   defaultQueue,
			"pending": fmt.Sprintf("%d", info.Pending),
			"active":  fmt.Sprintf("%d", info.Active),
			"retry":   fmt.Sprintf("%d", info.Retry),
		},
	}, nil
}

// checkFailureRatio fires when too large a share of today's processed tasks
// failed. This is the check that catches a worker which is up and consuming but
// failing everything it touches — the shape of the 2026-07-27/28 incident, where
// Servers() was healthy and 98% of tasks failed.
//
// Processed/Failed are asynq's DAILY counters and reset at midnight, so this
// reports "today", not a rolling window. Good enough: an incident this severe is
// visible within one tick of starting.
func checkFailureRatio(inspector queueInspector) (Finding, error) {
	info, err := inspector.GetQueueInfo(defaultQueue)
	if err != nil {
		return Finding{}, fmt.Errorf("inspect queue %q: %w", defaultQueue, err)
	}

	if info.Processed < failureRatioMinProcessed {
		return Finding{}, nil
	}

	ratio := float64(info.Failed) / float64(info.Processed)
	if ratio <= failureRatioThreshold {
		return Finding{}, nil
	}

	return Finding{
		Firing: true,
		Summary: fmt.Sprintf("%d of %d tasks failed today (%.0f%%, threshold %.0f%%).",
			info.Failed, info.Processed, ratio*100, failureRatioThreshold*100),
		Fields: map[string]string{
			"queue":    defaultQueue,
			"archived": fmt.Sprintf("%d", info.Archived),
			"retry":    fmt.Sprintf("%d", info.Retry),
		},
	}, nil
}

// archivedGrowthCheck fires when the archived count GREW since the last tick.
//
// Archiving is asynq's terminal give-up: a task that exhausted its retries and
// will never run again. In a healthy system that number never moves, so any
// increase is worth knowing about immediately — this is the most direct possible
// signal of "work was silently dropped".
//
// Level-vs-edge matters here. A level check ("archived > 0") would latch on
// forever, since asynq keeps archived tasks around until they are pruned or
// deleted by hand — one bad day would mean a permanently firing condition. So
// this is an EDGE check on the delta.
// Because it is an edge check, it goes healthy again on the first tick with no
// further growth — so a burst produces "Firing" and then, once archiving stops,
// "Resolved". That reads correctly: the resolve genuinely means "nothing more is
// being dropped". A sustained incident keeps growing and stays open.
type archivedGrowthCheck struct {
	last   int
	primed bool
}

func (c *archivedGrowthCheck) run(inspector queueInspector) (Finding, error) {
	info, err := inspector.GetQueueInfo(defaultQueue)
	if err != nil {
		return Finding{}, fmt.Errorf("inspect queue %q: %w", defaultQueue, err)
	}

	prev := c.last
	c.last = info.Archived

	// First observation establishes the baseline. Without this, a restart would
	// alert on the entire pre-existing archived backlog as if it just happened.
	if !c.primed {
		c.primed = true
		return Finding{}, nil
	}

	growth := info.Archived - prev
	if growth <= 0 {
		// Not an error when negative: asynq prunes archived tasks, and they can be
		// deleted from asynqmon. Re-baselining above already handled it.
		return Finding{}, nil
	}

	return Finding{
		Firing: true,
		Summary: fmt.Sprintf("%d task(s) were archived since the last check — they exhausted retries and will never run.",
			growth),
		Fields: map[string]string{
			"queue":          defaultQueue,
			"archived_total": fmt.Sprintf("%d", info.Archived),
			"hint":           "inspect with asynqmon; these tasks are lost unless re-run",
		},
	}, nil
}

// checkStuckSends fires when notifications are sitting unresolved.
//
// This is the backstop for a worker that is alive and reporting success while
// notifications never reach a terminal state. `enqueued` is the only
// non-terminal notification status, and it is normally transient (resolved
// sub-second by the notification:delivery task), so a nonzero count minutes
// later means the send path stalled after the initial INSERT.
func checkStuckSends(ctx context.Context, notifications stuckCounter, now func() time.Time) (Finding, error) {
	if notifications == nil {
		return Finding{}, nil
	}

	t := now()
	count, err := notifications.CountStuck(ctx, t.Add(-stuckSendAge), t.Add(-stuckSendWindow))
	if err != nil {
		return Finding{}, fmt.Errorf("count stuck notifications: %w", err)
	}

	if count == 0 {
		return Finding{}, nil
	}

	return Finding{
		Firing: true,
		Summary: fmt.Sprintf("%d notification(s) have been stuck in `enqueued` for over %s.",
			count, stuckSendAge),
		Fields: map[string]string{
			"window": stuckSendWindow.String(),
			"hint":   "the notification row was inserted but its delivery task never resolved",
		},
	}, nil
}
