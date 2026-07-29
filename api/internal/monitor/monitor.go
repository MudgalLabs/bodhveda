// Package monitor watches Bodhveda's own infrastructure and pushes alerts to
// Discord when it malfunctions.
//
// # Why this exists
//
// On 2026-07-27/28 the worker archived ~98% of its tasks for roughly a day
// (asynq `default`: 168 processed / 164 failed, 57 archived). Nothing noticed:
// the calling product kept reporting success, logs were clean, and no email
// reached a human. It was found by opening the console by hand. This package is
// the instrument that was missing.
//
// # ⚠️ Where this runs, and why it matters
//
// This MUST NOT run on the worker. A monitor enqueued as an Asynq task on the
// `default` queue dies in exactly the incident it exists to report — the same
// queue failure that drops the work also drops the alarm.
//
// So it runs as a ticker goroutine inside cmd/api: the API is the process that
// survives a worker crash, it already holds Redis and Postgres handles, and
// cmd/worker sets the precedent for this shape with runWebhookEventCleanup.
//
// The residual gap is the API process itself being down: nothing in here can
// report that, because a system cannot report its own death. That case is
// covered OUTSIDE the application by an inbound uptime monitor polling
// GET /ping — which also catches DNS and TLS failures this package is blind to.
package monitor

import (
	"context"
	"time"

	"github.com/mudgallabs/tantra/logger"
)

const (
	// DefaultInterval is how often the check set runs. Fast enough that an
	// incident is caught in minutes rather than a day; slow enough that the
	// Redis/Postgres reads are noise.
	DefaultInterval = 60 * time.Second

	// DefaultRenotifyAfter is how long an already-open condition stays quiet
	// before it is re-announced. At a 60s interval, alerting every tick would
	// post ~500 messages overnight, the channel would get muted, and the
	// alerting would silently stop working — which is the very failure this
	// package exists to prevent.
	DefaultRenotifyAfter = 30 * time.Minute
)

// Monitor runs a set of Checks on a ticker and pushes state transitions to a
// Sink.
type Monitor struct {
	checks   []Check
	tracker  *Tracker
	sink     Sink
	interval time.Duration
}

// Config is the wiring for New.
type Config struct {
	Checks []Check
	// Sink may be nil, which makes the monitor log-only. That is the deliberate
	// behaviour when BODHVEDA_ALERT_DISCORD_WEBHOOK_URL is unset, so local dev
	// and self-hosted stacks run every check without needing a Discord server.
	Sink          Sink
	Interval      time.Duration
	RenotifyAfter time.Duration
	// Now is injectable for tests. Nil means time.Now.
	Now func() time.Time
}

// New builds a Monitor, filling in defaults for zero-valued config.
func New(cfg Config) *Monitor {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.RenotifyAfter <= 0 {
		cfg.RenotifyAfter = DefaultRenotifyAfter
	}

	return &Monitor{
		checks:   cfg.Checks,
		tracker:  NewTracker(cfg.RenotifyAfter, cfg.Now),
		sink:     cfg.Sink,
		interval: cfg.Interval,
	}
}

// Run ticks until ctx is cancelled. It runs one cycle immediately so a
// misconfiguration (bad webhook URL, unreachable Redis) surfaces at startup
// rather than a minute later.
func (m *Monitor) Run(ctx context.Context) {
	l := logger.Get()

	if m.sink == nil {
		l.Infow("monitor: no alert sink configured, running in log-only mode",
			"interval", m.interval.String())
	} else {
		l.Infow("monitor started", "interval", m.interval.String(), "checks", len(m.checks))
	}

	m.Tick(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.Infow("monitor stopped")
			return
		case <-ticker.C:
			m.Tick(ctx)
		}
	}
}

// Tick runs every check once and dispatches any resulting alerts. Exported so
// tests can drive cycles deterministically instead of waiting on a ticker.
//
// It never returns an error and never panics out: a fault in the monitoring path
// must not take down the API process hosting it. A panicking check would
// otherwise kill the whole server, which would be a spectacular way for a
// reliability feature to cause an outage.
func (m *Monitor) Tick(ctx context.Context) {
	l := logger.Get()

	defer func() {
		if r := recover(); r != nil {
			l.Errorw("monitor: panic in check cycle (recovered)", "panic", r)
		}
	}()

	for _, check := range m.checks {
		finding, err := check.Run(ctx)
		if err != nil {
			// A check that cannot run is a gap in coverage, not an incident —
			// alerting on it would fire on every transient Redis hiccup. Log it and
			// move on; the heartbeat is what catches total loss.
			l.Errorw("monitor: check failed to run", "check", check.Name, "error", err)
			continue
		}

		alert := m.tracker.Observe(check.Name, check.MinConsecutive, finding)
		if alert == nil {
			continue
		}

		m.dispatch(ctx, *alert)
	}
}

func (m *Monitor) dispatch(ctx context.Context, alert Alert) {
	l := logger.Get()

	switch alert.Kind {
	case AlertResolved:
		l.Infow("monitor: condition resolved", "condition", alert.Condition, "since", alert.Since)
	default:
		l.Errorw("monitor: condition firing",
			"condition", alert.Condition, "kind", string(alert.Kind), "summary", alert.Summary)
	}

	if m.sink == nil {
		return
	}

	if err := m.sink.Send(ctx, alert); err != nil {
		// Never propagate. A failed alert delivery is bad, but taking the API down
		// over it would be worse.
		l.Errorw("monitor: failed to deliver alert", "condition", alert.Condition, "error", err)
	}
}
