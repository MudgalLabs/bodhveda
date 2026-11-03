package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// fakeInspector stands in for *asynq.Inspector so the checks are testable
// without a Redis.
type fakeInspector struct {
	servers    []*asynq.ServerInfo
	info       *asynq.QueueInfo
	serversErr error
	infoErr    error
}

func (f *fakeInspector) Servers() ([]*asynq.ServerInfo, error) {
	return f.servers, f.serversErr
}

func (f *fakeInspector) GetQueueInfo(string) (*asynq.QueueInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	if f.info == nil {
		return &asynq.QueueInfo{}, nil
	}
	return f.info, nil
}

type fakeStuckCounter struct {
	count int
	err   error
	// gotOlderThan/gotNewerThan capture the bounds the check passed down.
	gotOlderThan time.Time
	gotNewerThan time.Time
}

func (f *fakeStuckCounter) CountStuck(_ context.Context, olderThan, newerThan time.Time) (int, error) {
	f.gotOlderThan, f.gotNewerThan = olderThan, newerThan
	return f.count, f.err
}

func TestCheckWorkerAbsent(t *testing.T) {
	t.Run("fires when no server is registered", func(t *testing.T) {
		f, err := checkWorkerAbsent(&fakeInspector{servers: nil})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !f.Firing {
			t.Fatal("no registered worker should fire")
		}
	})

	t.Run("silent when a server is registered", func(t *testing.T) {
		f, err := checkWorkerAbsent(&fakeInspector{servers: []*asynq.ServerInfo{{ID: "w1"}}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Firing {
			t.Fatal("a registered worker should not fire")
		}
	})

	t.Run("propagates inspector errors instead of firing", func(t *testing.T) {
		// A check that cannot run is a coverage gap, not an incident. Reporting it
		// as Firing would alert on every transient Redis hiccup.
		f, err := checkWorkerAbsent(&fakeInspector{serversErr: errors.New("redis down")})
		if err == nil {
			t.Fatal("expected the inspector error to propagate")
		}
		if f.Firing {
			t.Fatal("an errored check must not report as firing")
		}
	})
}

func TestCheckQueueLatency(t *testing.T) {
	tests := []struct {
		name    string
		latency time.Duration
		want    bool
	}{
		{"empty queue reports zero latency", 0, false},
		{"healthy", 3 * time.Second, false},
		{"exactly at threshold is not over it", queueLatencyThreshold, false},
		{"backed up", queueLatencyThreshold + time.Second, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := checkQueueLatency(&fakeInspector{info: &asynq.QueueInfo{Latency: tc.latency}})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.Firing != tc.want {
				t.Errorf("latency %s: firing = %v, want %v", tc.latency, f.Firing, tc.want)
			}
		})
	}
}

func TestFailureRatioCheck(t *testing.T) {
	// Each case is a baseline tick then one tick a minute later; the check judges
	// only what happened between them.
	tests := []struct {
		name              string
		processed, failed int // growth between the two ticks
		want              bool
	}{
		{
			// The incident this whole package exists for: the worker was UP and
			// consuming, so worker_absent stayed silent. This is the check that
			// catches it.
			name: "the 2026-07-27 incident", processed: 164, failed: 160, want: true,
		},
		{
			// Broadcast 4: nine batches timed out together on a quiet night. Low
			// traffic must not hide a real incident.
			name: "the 2026-11-03 broadcast stall", processed: 11, failed: 9, want: true,
		},
		{name: "healthy", processed: 500, failed: 3, want: false},
		{name: "a lone failure is not an incident", processed: 1, failed: 1, want: false},
		{name: "just below the minimum failures", processed: failureRatioMinFailed - 1, failed: failureRatioMinFailed - 1, want: false},
		{name: "at the minimum failures, over threshold", processed: failureRatioMinFailed, failed: failureRatioMinFailed, want: true},
		{name: "at the threshold exactly is not over it", processed: 100, failed: 25, want: false},
		{name: "just over the threshold", processed: 100, failed: 26, want: true},
		{name: "nothing processed", processed: 0, failed: 0, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
			c := &failureRatioCheck{window: time.Hour, now: func() time.Time { return clock }}
			insp := &fakeInspector{info: &asynq.QueueInfo{Processed: 1000, Failed: 10}}

			if f, err := c.run(insp); err != nil || f.Firing {
				t.Fatalf("the baseline tick must never fire: firing=%v err=%v", f.Firing, err)
			}

			clock = clock.Add(time.Minute)
			insp.info = &asynq.QueueInfo{Processed: 1000 + tc.processed, Failed: 10 + tc.failed}
			f, err := c.run(insp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.Firing != tc.want {
				t.Errorf("%d/%d: firing = %v, want %v", tc.failed, tc.processed, f.Firing, tc.want)
			}
		})
	}
}

// The two faults of the daily-counter version, both seen on 2026-11-03.
func TestFailureRatioCheckWindow(t *testing.T) {
	start := time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	clock := start
	newCheck := func() *failureRatioCheck {
		return &failureRatioCheck{window: time.Hour, now: func() time.Time { return clock }}
	}
	tick := func(c *failureRatioCheck, processed, failed int) bool {
		t.Helper()
		f, err := c.run(&fakeInspector{info: &asynq.QueueInfo{Processed: processed, Failed: failed}})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return f.Firing
	}

	t.Run("resolves once failures age out of the window", func(t *testing.T) {
		clock = start
		c := newCheck()
		tick(c, 15, 0)
		clock = clock.Add(time.Minute)
		if !tick(c, 24, 9) {
			t.Fatal("nine failures in a burst must fire")
		}

		// Nothing else runs afterwards, so only time can clear it. Under the daily
		// counters "9 of 31" stayed over 25% for the rest of the day; here the
		// burst must keep counting while it is inside the window and clear after.
		clock = clock.Add(30 * time.Minute)
		if !tick(c, 24, 9) {
			t.Fatal("still inside the window, the burst should still count")
		}
		clock = clock.Add(31 * time.Minute)
		if tick(c, 24, 9) {
			t.Error("once the burst is older than the window the condition must resolve")
		}
	})

	t.Run("a restart does not re-announce old failures", func(t *testing.T) {
		clock = start
		// Fresh process after a deploy: the day's counters already hold the burst.
		c := newCheck()
		if tick(c, 31, 9) {
			t.Fatal("the first tick after boot is a baseline, not an alert")
		}
		clock = clock.Add(time.Minute)
		if tick(c, 32, 9) {
			t.Error("no new failures since boot, so nothing to say")
		}
	})

	t.Run("a restart mid-incident still fires on the next tick", func(t *testing.T) {
		clock = start
		c := newCheck()
		tick(c, 31, 9)
		clock = clock.Add(time.Minute)
		if !tick(c, 41, 18) {
			t.Error("failures continuing after boot must fire")
		}
	})

	t.Run("midnight reset counts from zero", func(t *testing.T) {
		clock = start
		c := newCheck()
		tick(c, 400, 2)
		clock = clock.Add(time.Minute)
		// Counters reset, and the first minute of the new day is all failures.
		if !tick(c, 8, 8) {
			t.Error("the reset must rebaseline at zero, not swallow the new day's failures")
		}
	})
}

func TestArchivedGrowthCheck(t *testing.T) {
	t.Run("first observation only establishes a baseline", func(t *testing.T) {
		// Otherwise a restart would alert on the entire pre-existing backlog as
		// though it had just happened.
		c := &archivedGrowthCheck{}
		f, err := c.run(&fakeInspector{info: &asynq.QueueInfo{Archived: 57}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Firing {
			t.Fatal("the priming observation must not fire")
		}
	})

	t.Run("fires on growth", func(t *testing.T) {
		c := &archivedGrowthCheck{}
		insp := &fakeInspector{info: &asynq.QueueInfo{Archived: 10}}
		if _, err := c.run(insp); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		insp.info = &asynq.QueueInfo{Archived: 14}
		f, err := c.run(insp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !f.Firing {
			t.Fatal("an increase in archived tasks should fire")
		}
	})

	t.Run("does not latch on a steady non-zero count", func(t *testing.T) {
		// A level check would stay firing forever, since asynq keeps archived
		// tasks until they are pruned or deleted by hand. This is why it is an
		// EDGE check.
		c := &archivedGrowthCheck{}
		insp := &fakeInspector{info: &asynq.QueueInfo{Archived: 57}}

		for i := 0; i < 3; i++ {
			f, err := c.run(insp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.Firing {
				t.Fatalf("tick %d: a steady archived count must not fire", i)
			}
		}
	})

	t.Run("handles the count decreasing", func(t *testing.T) {
		// Archived tasks get pruned, and can be deleted from asynqmon.
		c := &archivedGrowthCheck{}
		insp := &fakeInspector{info: &asynq.QueueInfo{Archived: 57}}
		if _, err := c.run(insp); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		insp.info = &asynq.QueueInfo{Archived: 0}
		f, err := c.run(insp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Firing {
			t.Fatal("a decrease must not fire")
		}

		// And the baseline must have moved down, so the next increase is measured
		// from 0 rather than from the stale 57.
		insp.info = &asynq.QueueInfo{Archived: 2}
		f, err = c.run(insp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !f.Firing {
			t.Fatal("growth after a decrease should fire against the new baseline")
		}
	})
}

func TestCheckStuckSends(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) }

	t.Run("fires when notifications are stuck", func(t *testing.T) {
		f, err := checkStuckSends(context.Background(), &fakeStuckCounter{count: 12}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !f.Firing {
			t.Fatal("stuck notifications should fire")
		}
	})

	t.Run("silent when nothing is stuck", func(t *testing.T) {
		f, err := checkStuckSends(context.Background(), &fakeStuckCounter{count: 0}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Firing {
			t.Fatal("zero stuck notifications must not fire")
		}
	})

	t.Run("passes a bounded lookback window", func(t *testing.T) {
		// The newerThan bound is what stops this query degrading into a
		// full-history scan as the notification table grows.
		counter := &fakeStuckCounter{}
		if _, err := checkStuckSends(context.Background(), counter, now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, want := counter.gotOlderThan, now().Add(-stuckSendAge); !got.Equal(want) {
			t.Errorf("olderThan = %v, want %v", got, want)
		}
		if got, want := counter.gotNewerThan, now().Add(-stuckSendWindow); !got.Equal(want) {
			t.Errorf("newerThan = %v, want %v", got, want)
		}
		if !counter.gotNewerThan.Before(counter.gotOlderThan) {
			t.Error("the window bounds are inverted; the query would match nothing")
		}
	})

	t.Run("propagates repository errors instead of firing", func(t *testing.T) {
		f, err := checkStuckSends(context.Background(), &fakeStuckCounter{err: errors.New("db down")}, now)
		if err == nil {
			t.Fatal("expected the repository error to propagate")
		}
		if f.Firing {
			t.Fatal("an errored check must not report as firing")
		}
	})
}

// The check set must cover BOTH failure modes. Trimming it to "is the worker
// alive?" would miss the incident that motivated the package, where the worker
// was running the whole time.
func TestDefaultChecksCoverBothFailureModes(t *testing.T) {
	checks := DefaultChecks(&fakeInspector{}, &fakeStuckCounter{}, nil)

	byName := make(map[string]Check, len(checks))
	for _, c := range checks {
		byName[c.Name] = c
	}

	for _, name := range []string{
		"worker_absent", "queue_backed_up", // worker crashed / not consuming
		"task_failure_ratio", "tasks_archived", // worker alive, failing everything
		"stuck_sends", // alive and reporting success, nothing resolves
	} {
		if _, ok := byName[name]; !ok {
			t.Errorf("check %q is missing from the default set", name)
		}
	}

	if got := byName["task_failure_ratio"].MinConsecutive; got != 1 {
		t.Errorf("task_failure_ratio should alert on the first bad reading, got MinConsecutive=%d", got)
	}
	if got := byName["worker_absent"].MinConsecutive; got < 2 {
		t.Errorf("worker_absent should tolerate a restart gap, got MinConsecutive=%d", got)
	}
}

// End-to-end at the check layer: replay the incident and assert the two checks
// that should have caught it actually do.
func TestIncidentReplayFiresTheRightChecks(t *testing.T) {
	insp := &fakeInspector{
		// The worker was UP the whole time — it was consuming and archiving.
		servers: []*asynq.ServerInfo{{ID: "worker-1"}},
		// Baseline: the moment before it went wrong.
		info: &asynq.QueueInfo{Processed: 4, Failed: 0, Archived: 0},
	}
	clock := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	checks := DefaultChecks(insp, &fakeStuckCounter{count: 0}, func() time.Time { return clock })

	run := func() map[string]bool {
		t.Helper()
		out := make(map[string]bool)
		for _, c := range checks {
			f, err := c.Run(context.Background())
			if err != nil {
				t.Fatalf("check %q errored: %v", c.Name, err)
			}
			out[c.Name] = f.Firing
		}
		return out
	}

	// First cycle primes the archived and failure-ratio baselines.
	if first := run(); first["worker_absent"] {
		t.Error("worker_absent must stay silent — the worker was running during the incident")
	}

	// Next cycle: 164 of the next 164 tasks failed.
	clock = clock.Add(time.Minute)
	insp.info = &asynq.QueueInfo{Processed: 168, Failed: 164, Archived: 0}
	first := run()
	if first["worker_absent"] {
		t.Error("worker_absent must stay silent — the worker was running during the incident")
	}
	if !first["task_failure_ratio"] {
		t.Error("task_failure_ratio must fire on 164/168")
	}

	// Next cycle, still failing, with tasks now being archived.
	clock = clock.Add(time.Minute)
	insp.info = &asynq.QueueInfo{Processed: 200, Failed: 196, Archived: 57}
	second := run()
	if !second["tasks_archived"] {
		t.Error("tasks_archived must fire once the archived count grows")
	}
	if !second["task_failure_ratio"] {
		t.Error("task_failure_ratio must still fire")
	}
}
