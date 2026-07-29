package monitor

import (
	"testing"
	"time"
)

// fakeClock drives Tracker's time so the re-notify window can be tested without
// sleeping.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func firing(summary string) Finding {
	return Finding{Firing: true, Summary: summary}
}

func healthy() Finding { return Finding{} }

// The core contract: one alert when a condition opens, one when it resolves, and
// SILENCE in between. Everything else in this file is a variation on why that
// silence has to hold.
func TestTrackerFiresOnceOnOpenAndOnceOnResolve(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)}
	tr := NewTracker(30*time.Minute, clock.now)

	alert := tr.Observe("worker_absent", 1, firing("no worker"))
	if alert == nil || alert.Kind != AlertOpened {
		t.Fatalf("first firing tick should open the condition, got %+v", alert)
	}

	// Still broken, but inside the quiet window — must stay silent.
	for i := 0; i < 5; i++ {
		clock.advance(time.Minute)
		if a := tr.Observe("worker_absent", 1, firing("no worker")); a != nil {
			t.Fatalf("tick %d inside the quiet window should be silent, got %+v", i, a)
		}
	}

	clock.advance(time.Minute)
	alert = tr.Observe("worker_absent", 1, healthy())
	if alert == nil || alert.Kind != AlertResolved {
		t.Fatalf("recovery should emit a resolve, got %+v", alert)
	}

	// Healthy and already resolved — nothing more to say.
	if a := tr.Observe("worker_absent", 1, healthy()); a != nil {
		t.Fatalf("steady-state healthy should be silent, got %+v", a)
	}
}

// The whole point of the quiet window: at a 60s interval, alerting every tick
// during an overnight outage is ~500 messages, the channel gets muted, and the
// alerting silently stops working.
func TestTrackerRenotifiesOnlyAfterQuietWindow(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)}
	tr := NewTracker(30*time.Minute, clock.now)

	if a := tr.Observe("queue_backed_up", 1, firing("backed up")); a == nil {
		t.Fatal("expected the condition to open")
	}

	// 29 minutes of one-minute ticks: all silent.
	alerts := 0
	for i := 0; i < 29; i++ {
		clock.advance(time.Minute)
		if a := tr.Observe("queue_backed_up", 1, firing("backed up")); a != nil {
			alerts++
		}
	}
	if alerts != 0 {
		t.Fatalf("expected silence for 29 minutes, got %d alerts", alerts)
	}

	// Crossing 30 minutes re-announces exactly once.
	clock.advance(time.Minute)
	alert := tr.Observe("queue_backed_up", 1, firing("backed up"))
	if alert == nil || alert.Kind != AlertOngoing {
		t.Fatalf("expected an ongoing re-notify after the quiet window, got %+v", alert)
	}

	clock.advance(time.Minute)
	if a := tr.Observe("queue_backed_up", 1, firing("backed up")); a != nil {
		t.Fatalf("the window should have reset after re-notifying, got %+v", a)
	}
}

func TestTrackerRequiresMinConsecutiveBeforeOpening(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)}
	tr := NewTracker(30*time.Minute, clock.now)

	if a := tr.Observe("worker_absent", 2, firing("no worker")); a != nil {
		t.Fatalf("one bad tick must not open a MinConsecutive=2 condition, got %+v", a)
	}
	if tr.IsOpen("worker_absent") {
		t.Fatal("condition should not be open after a single unhealthy tick")
	}

	clock.advance(time.Minute)
	alert := tr.Observe("worker_absent", 2, firing("no worker"))
	if alert == nil || alert.Kind != AlertOpened {
		t.Fatalf("the second consecutive bad tick should open it, got %+v", alert)
	}
}

// A flapping check must not accumulate its way to open over hours. This is why
// a healthy tick resets the streak rather than decrementing it.
func TestTrackerResetsStreakOnHealthyTick(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)}
	tr := NewTracker(30*time.Minute, clock.now)

	for i := 0; i < 10; i++ {
		if a := tr.Observe("queue_backed_up", 3, firing("blip")); a != nil {
			t.Fatalf("alternating tick %d should never open the condition, got %+v", i, a)
		}
		clock.advance(time.Minute)

		if a := tr.Observe("queue_backed_up", 3, healthy()); a != nil {
			t.Fatalf("healthy tick %d should be silent, got %+v", i, a)
		}
		clock.advance(time.Minute)
	}

	if tr.IsOpen("queue_backed_up") {
		t.Fatal("a flapping check must never reach the open state")
	}
}

// A never-fired condition going healthy is not a recovery — there was nothing to
// recover from. Without this, every check would post a "Resolved" on boot.
func TestTrackerDoesNotResolveAConditionThatNeverOpened(t *testing.T) {
	tr := NewTracker(30*time.Minute, nil)

	if a := tr.Observe("stuck_sends", 1, healthy()); a != nil {
		t.Fatalf("a healthy first observation must be silent, got %+v", a)
	}
}

// A condition that fires but never reaches MinConsecutive has not opened, so its
// recovery must also stay silent.
func TestTrackerDoesNotResolveBelowMinConsecutive(t *testing.T) {
	tr := NewTracker(30*time.Minute, nil)

	if a := tr.Observe("worker_absent", 2, firing("no worker")); a != nil {
		t.Fatalf("expected silence below the threshold, got %+v", a)
	}
	if a := tr.Observe("worker_absent", 2, healthy()); a != nil {
		t.Fatalf("recovering from a sub-threshold blip must be silent, got %+v", a)
	}
}

func TestTrackerKeepsConditionsIndependent(t *testing.T) {
	tr := NewTracker(30*time.Minute, nil)

	if a := tr.Observe("worker_absent", 1, firing("no worker")); a == nil {
		t.Fatal("expected worker_absent to open")
	}
	if a := tr.Observe("stuck_sends", 1, healthy()); a != nil {
		t.Fatalf("an unrelated healthy condition must not be affected, got %+v", a)
	}

	if !tr.IsOpen("worker_absent") {
		t.Fatal("worker_absent should still be open")
	}
	if tr.IsOpen("stuck_sends") {
		t.Fatal("stuck_sends should never have opened")
	}
}

// A resolve reports the summary that was true while broken, and the time it
// started — a recovered check has no current numbers to show.
func TestResolvedAlertCarriesOpeningContext(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)}
	tr := NewTracker(30*time.Minute, clock.now)

	opened := tr.Observe("task_failure_ratio", 1, Finding{
		Firing:  true,
		Summary: "164 of 168 tasks failed today (98%).",
		Fields:  map[string]string{"queue": "default"},
	})
	if opened == nil {
		t.Fatal("expected the condition to open")
	}

	clock.advance(2 * time.Hour)
	resolved := tr.Observe("task_failure_ratio", 1, healthy())
	if resolved == nil {
		t.Fatal("expected a resolve")
	}

	if resolved.Summary != "164 of 168 tasks failed today (98%)." {
		t.Errorf("resolve should carry the summary from while it was broken, got %q", resolved.Summary)
	}
	if !resolved.Since.Equal(opened.Since) {
		t.Errorf("resolve Since = %v, want the open time %v", resolved.Since, opened.Since)
	}
}

func TestAlertFieldLinesAreSorted(t *testing.T) {
	a := Alert{Fields: map[string]string{"queue": "default", "archived": "57", "retry": "3"}}

	lines := a.FieldLines()
	want := []string{"archived: 57", "queue: default", "retry: 3"}

	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestAlertTitleReflectsKind(t *testing.T) {
	tests := []struct {
		kind AlertKind
		want string
	}{
		{AlertOpened, "Firing: worker absent"},
		{AlertOngoing, "Still firing: worker absent"},
		{AlertResolved, "Resolved: worker absent"},
	}

	for _, tc := range tests {
		got := Alert{Condition: "worker_absent", Kind: tc.kind}.Title()
		if got != tc.want {
			t.Errorf("kind %q title = %q, want %q", tc.kind, got, tc.want)
		}
	}
}
