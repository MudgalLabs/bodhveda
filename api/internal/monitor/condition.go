package monitor

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// AlertKind is what happened to a condition, and therefore what the alert is
// telling the reader.
type AlertKind string

const (
	// AlertOpened — the condition just started firing.
	AlertOpened AlertKind = "opened"
	// AlertOngoing — it is STILL firing, re-notified after the quiet window.
	AlertOngoing AlertKind = "ongoing"
	// AlertResolved — it stopped firing.
	AlertResolved AlertKind = "resolved"
)

// Finding is what one check reports for one tick.
type Finding struct {
	// Firing is whether the check considers the system unhealthy right now.
	Firing bool
	// Summary is the one-line human description that lands in Discord. It should
	// carry the actual numbers ("164 of 168 tasks failed"), not just the
	// condition name — a reader must be able to judge severity without opening
	// anything.
	Summary string
	// Fields are optional extra key/values rendered under the summary.
	Fields map[string]string
}

// Alert is a state TRANSITION worth pushing, produced by Tracker.Observe.
type Alert struct {
	Condition string
	Kind      AlertKind
	// Summary on an opened/ongoing alert is the CURRENT state. On a resolved
	// alert it is the state that was true WHILE BROKEN — a recovered check has no
	// current numbers to report. Renderers must present it in the past tense; see
	// buildPayload, where a resolve that reads like a live problem was a real bug.
	Summary string
	Fields  map[string]string
	// Since is when the condition opened.
	Since time.Time
	// ResolvedAfter is how long the condition stayed open. Zero on anything but a
	// resolve.
	ResolvedAfter time.Duration
}

// FieldLines renders Fields as stable, sorted "key: value" lines. Sorted because
// Go map iteration is random and an alert whose lines shuffle between messages
// is needlessly hard to diff by eye.
func (a Alert) FieldLines() []string {
	if len(a.Fields) == 0 {
		return nil
	}

	keys := make([]string, 0, len(a.Fields))
	for k := range a.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", k, a.Fields[k]))
	}
	return lines
}

// Title is the alert's headline.
func (a Alert) Title() string {
	switch a.Kind {
	case AlertResolved:
		return "Resolved: " + humanize(a.Condition)
	case AlertOngoing:
		return "Still firing: " + humanize(a.Condition)
	default:
		return "Firing: " + humanize(a.Condition)
	}
}

func humanize(condition string) string {
	return strings.ReplaceAll(condition, "_", " ")
}

// conditionState is the per-condition memory the Tracker keeps between ticks.
type conditionState struct {
	// open is whether the condition has crossed minConsecutive and is currently
	// alerting.
	open bool
	// consecutive counts unhealthy ticks in a row. Reset by any healthy tick, so
	// a check with minConsecutive > 1 only opens on a SUSTAINED problem.
	consecutive int
	since       time.Time
	// lastNotified drives the re-notify quiet window.
	lastNotified time.Time
	summary      string
	fields       map[string]string
}

// Tracker turns a stream of per-tick Findings into the much smaller stream of
// transitions worth pushing to Discord.
//
// This exists because the naive version — post whenever a check is unhealthy —
// posts every tick for as long as the incident lasts. At a 60s interval an
// overnight outage is ~500 messages, the channel gets muted, and the alerting
// silently stops working. That failure mode is the whole reason this feature
// exists, so the hygiene is not a nicety.
//
// State is in memory and per-process. An API restart re-opens conditions that
// are still true and re-alerts, which is acceptable and arguably correct: a
// restart during an incident is worth a message.
//
// Not safe for concurrent use — the monitor drives it from a single goroutine.
type Tracker struct {
	states map[string]*conditionState
	// renotifyAfter is how long a condition must have been continuously open
	// before it is re-announced.
	renotifyAfter time.Duration
	// now is injectable so tests can drive the clock instead of sleeping.
	now func() time.Time
}

// NewTracker builds a Tracker. Pass nil for now to use time.Now.
func NewTracker(renotifyAfter time.Duration, now func() time.Time) *Tracker {
	if now == nil {
		now = time.Now
	}
	return &Tracker{
		states:        make(map[string]*conditionState),
		renotifyAfter: renotifyAfter,
		now:           now,
	}
}

// Observe folds one tick's Finding for `condition` into the tracker and returns
// the Alert to push, or nil when this tick changes nothing worth saying.
//
// minConsecutive is how many unhealthy ticks in a row are required before the
// condition opens — the per-check knob for "is one bad reading enough?". A
// value < 1 is treated as 1.
func (t *Tracker) Observe(condition string, minConsecutive int, f Finding) *Alert {
	if minConsecutive < 1 {
		minConsecutive = 1
	}

	now := t.now()

	st, ok := t.states[condition]
	if !ok {
		st = &conditionState{}
		t.states[condition] = st
	}

	if !f.Firing {
		wasOpen := st.open

		// Reset fully: a healthy tick clears the streak, so an intermittent check
		// never accumulates its way to open over hours.
		st.open = false
		st.consecutive = 0

		if !wasOpen {
			return nil
		}

		alert := &Alert{
			Condition: condition,
			Kind:      AlertResolved,
			// Report the summary that was true while it was broken; a resolved
			// check has no current numbers to show.
			Summary:       st.summary,
			Fields:        st.fields,
			Since:         st.since,
			ResolvedAfter: now.Sub(st.since),
		}
		st.summary = ""
		st.fields = nil
		return alert
	}

	st.consecutive++
	st.summary = f.Summary
	st.fields = f.Fields

	// Firing, but not yet for long enough to count.
	if st.consecutive < minConsecutive {
		return nil
	}

	if !st.open {
		st.open = true
		st.since = now
		st.lastNotified = now

		return &Alert{
			Condition: condition,
			Kind:      AlertOpened,
			Summary:   f.Summary,
			Fields:    f.Fields,
			Since:     now,
		}
	}

	// Already open — stay quiet until the re-notify window elapses.
	if now.Sub(st.lastNotified) < t.renotifyAfter {
		return nil
	}
	st.lastNotified = now

	return &Alert{
		Condition: condition,
		Kind:      AlertOngoing,
		Summary:   f.Summary,
		Fields:    f.Fields,
		Since:     st.since,
	}
}

// IsOpen reports whether a condition is currently open. Test/debug helper.
func (t *Tracker) IsOpen(condition string) bool {
	st, ok := t.states[condition]
	return ok && st.open
}
