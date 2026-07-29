package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingSink captures dispatched alerts.
type recordingSink struct {
	mu     sync.Mutex
	alerts []Alert
	err    error
}

func (s *recordingSink) Send(_ context.Context, a Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, a)
	return s.err
}

func (s *recordingSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.alerts)
}

func staticCheck(name string, f *Finding) Check {
	return Check{
		Name:           name,
		MinConsecutive: 1,
		Run:            func(context.Context) (Finding, error) { return *f, nil },
	}
}

func TestMonitorDispatchesOpenAndResolve(t *testing.T) {
	finding := firing("worker is gone")
	sink := &recordingSink{}

	m := New(Config{
		Checks: []Check{staticCheck("worker_absent", &finding)},
		Sink:   sink,
	})

	m.Tick(context.Background())
	if sink.count() != 1 {
		t.Fatalf("expected 1 alert after the condition opened, got %d", sink.count())
	}

	// Still broken and inside the quiet window.
	m.Tick(context.Background())
	if sink.count() != 1 {
		t.Fatalf("expected no further alerts inside the quiet window, got %d", sink.count())
	}

	finding = healthy()
	m.Tick(context.Background())
	if sink.count() != 2 {
		t.Fatalf("expected a resolve alert, got %d total", sink.count())
	}
	if got := sink.alerts[1].Kind; got != AlertResolved {
		t.Errorf("second alert kind = %q, want %q", got, AlertResolved)
	}
}

// A monitoring bug must never take down the API process it runs inside — that
// would be a spectacular way for a reliability feature to cause an outage.
func TestMonitorSurvivesAPanickingCheck(t *testing.T) {
	sink := &recordingSink{}
	good := firing("still reported")

	m := New(Config{
		Checks: []Check{
			{
				Name:           "exploding",
				MinConsecutive: 1,
				Run: func(context.Context) (Finding, error) {
					panic("boom")
				},
			},
			staticCheck("healthy_check", &good),
		},
		Sink: sink,
	})

	// Must not panic out of Tick.
	m.Tick(context.Background())
}

// A check that cannot run is a gap in coverage, not an incident. Alerting on it
// would fire on every transient Redis hiccup.
func TestMonitorLogsCheckErrorsWithoutAlerting(t *testing.T) {
	sink := &recordingSink{}

	m := New(Config{
		Checks: []Check{{
			Name:           "broken",
			MinConsecutive: 1,
			Run:            func(context.Context) (Finding, error) { return Finding{}, errors.New("redis down") },
		}},
		Sink: sink,
	})

	m.Tick(context.Background())

	if sink.count() != 0 {
		t.Fatalf("an errored check must not produce an alert, got %d", sink.count())
	}
}

// The documented behaviour when BODHVEDA_ALERT_DISCORD_WEBHOOK_URL is unset:
// every check still runs, findings are logged, nothing is pushed, nothing
// crashes. This is what makes local dev work without a Discord server.
func TestMonitorRunsLogOnlyWithoutASink(t *testing.T) {
	finding := firing("worker is gone")
	ran := 0

	m := New(Config{
		Checks: []Check{{
			Name:           "worker_absent",
			MinConsecutive: 1,
			Run: func(context.Context) (Finding, error) {
				ran++
				return finding, nil
			},
		}},
		Sink: nil,
	})

	m.Tick(context.Background())

	if ran != 1 {
		t.Fatalf("check should have run once, ran %d times", ran)
	}
}

// Regression guard for the typed-nil-into-interface trap: NewDiscordSink returns
// the Sink interface precisely so an empty URL yields a genuinely nil interface.
// If it returned *DiscordSink, this would be a non-nil interface holding a nil
// pointer, and the first alert would panic on a nil receiver — turning "no
// Discord configured" into a crash on the first real incident.
func TestEmptyDiscordURLYieldsATrulyNilSink(t *testing.T) {
	if sink := NewDiscordSink(""); sink != nil {
		t.Fatal("an empty webhook URL must yield a nil Sink interface, not a typed nil")
	}
	if sink := NewDiscordSink("   "); sink != nil {
		t.Fatal("a blank webhook URL must yield a nil Sink interface")
	}

	finding := firing("worker is gone")
	m := New(Config{
		Checks: []Check{staticCheck("worker_absent", &finding)},
		Sink:   NewDiscordSink(""),
	})

	// Would panic if the nil-interface handling were wrong.
	m.Tick(context.Background())
}

// A broken alert channel is bad; taking the API down over it would be worse.
func TestMonitorSurvivesASinkFailure(t *testing.T) {
	finding := firing("worker is gone")
	sink := &recordingSink{err: errors.New("discord 500")}

	m := New(Config{
		Checks: []Check{staticCheck("worker_absent", &finding)},
		Sink:   sink,
	})

	m.Tick(context.Background())

	if sink.count() != 1 {
		t.Fatalf("the sink should still have been called, got %d", sink.count())
	}
}

func TestMonitorRunStopsOnContextCancel(t *testing.T) {
	finding := healthy()
	m := New(Config{
		Checks:   []Check{staticCheck("worker_absent", &finding)},
		Interval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		m.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	m := New(Config{})

	if m.interval != DefaultInterval {
		t.Errorf("interval = %v, want %v", m.interval, DefaultInterval)
	}
	if m.tracker.renotifyAfter != DefaultRenotifyAfter {
		t.Errorf("renotifyAfter = %v, want %v", m.tracker.renotifyAfter, DefaultRenotifyAfter)
	}
}

func TestDiscordSinkPostsAnEmbed(t *testing.T) {
	var mu sync.Mutex
	var body []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		// io.ReadAll, not a single Read: one Read may return a partial body.
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sink := NewDiscordSink(srv.URL)
	err := sink.Send(context.Background(), Alert{
		Condition: "task_failure_ratio",
		Kind:      AlertOpened,
		Summary:   "164 of 168 tasks failed today (98%).",
		Fields:    map[string]string{"queue": "default", "archived": "57"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var payload discordPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("posted body is not valid JSON: %v (%s)", err, body)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]
	if embed.Title != "Firing: task failure ratio" {
		t.Errorf("title = %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "164 of 168") {
		t.Errorf("description should carry the summary numbers, got %q", embed.Description)
	}
	// Fields land in a sorted code block so successive messages diff cleanly.
	if !strings.Contains(embed.Description, "archived: 57") {
		t.Errorf("description should carry the fields, got %q", embed.Description)
	}
	if embed.Color != colourRed {
		t.Errorf("an opened alert should be red, got %d", embed.Color)
	}
}

func TestDiscordSinkReportsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Invalid Webhook Token"}`))
	}))
	defer srv.Close()

	err := NewDiscordSink(srv.URL).Send(context.Background(), Alert{Condition: "x", Kind: AlertOpened})
	if err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should name the status code, got %q", err)
	}
}

// Regression guard for a bug caught by looking at the rendered Discord card: the
// resolve echoed the broken-state summary AND its action hint, producing a green
// "Resolved: worker absent" that read "No Asynq worker is registered — check the
// worker container/process". That tells the reader to go fix something already
// fixed, which is exactly the kind of noise that gets a channel muted.
func TestResolvedEmbedDoesNotReadLikeALiveProblem(t *testing.T) {
	embed := buildPayload(Alert{
		Condition:     "worker_absent",
		Kind:          AlertResolved,
		Summary:       "No Asynq worker is registered — nothing is consuming the queue.",
		Fields:        map[string]string{"hint": "check the worker container/process", "queue": "default"},
		Since:         time.Date(2026, 7, 29, 10, 47, 45, 0, time.UTC),
		ResolvedAfter: 92 * time.Second,
	}).Embeds[0]

	if !strings.HasPrefix(embed.Description, "Recovered after 1m.") {
		t.Errorf("a resolve must LEAD with the recovery, got %q", embed.Description)
	}

	// The broken-state summary may still appear, but only marked as past tense.
	if !strings.Contains(embed.Description, "Was: No Asynq worker is registered") {
		t.Errorf("the prior summary should be present and past-tense, got %q", embed.Description)
	}

	// Action hints are for a problem that no longer exists.
	if strings.Contains(embed.Description, "check the worker container") {
		t.Errorf("a resolve must not repeat action hints, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "```") {
		t.Errorf("a resolve must not render the broken-state field block, got %q", embed.Description)
	}

	if embed.Color != colourGreen {
		t.Errorf("resolve colour = %d, want green %d", embed.Color, colourGreen)
	}
}

// An opened alert is the opposite case: it SHOULD carry the hints and fields,
// because those are what you act on.
func TestOpenedEmbedKeepsFieldsAndHints(t *testing.T) {
	embed := buildPayload(Alert{
		Condition: "worker_absent",
		Kind:      AlertOpened,
		Summary:   "No Asynq worker is registered — nothing is consuming the queue.",
		Fields:    map[string]string{"hint": "check the worker container/process", "queue": "default"},
	}).Embeds[0]

	if !strings.Contains(embed.Description, "check the worker container/process") {
		t.Errorf("an opened alert must carry its action hint, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Recovered after") {
		t.Errorf("an opened alert must not claim recovery, got %q", embed.Description)
	}
}

func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "under a second"},
		{3 * time.Second, "3s"},
		{59 * time.Second, "59s"},
		{92 * time.Second, "1m"},
		{45 * time.Minute, "45m"},
		{2 * time.Hour, "2h"},
		{150 * time.Minute, "2h 30m"},
	}

	for _, tc := range tests {
		if got := humanizeDuration(tc.d); got != tc.want {
			t.Errorf("humanizeDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDiscordSinkColoursByKind(t *testing.T) {
	tests := []struct {
		kind AlertKind
		want int
	}{
		{AlertOpened, colourRed},
		{AlertOngoing, colourAmber},
		{AlertResolved, colourGreen},
	}

	for _, tc := range tests {
		got := buildPayload(Alert{Kind: tc.kind}).Embeds[0].Color
		if got != tc.want {
			t.Errorf("kind %q colour = %d, want %d", tc.kind, got, tc.want)
		}
	}
}
