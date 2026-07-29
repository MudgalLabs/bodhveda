//go:build smoke

// Manual smoke test for the infra monitor. Excluded from the normal suite by the
// `smoke` build tag because it makes REAL network calls: it talks to the live
// Redis and posts real messages to the configured Discord webhook.
//
//	go test ./internal/monitor/ -tags smoke -run TestSmoke -v
//
// It drives the whole path end to end — real check, real Tracker transition,
// real Discord POST — and resolves the condition genuinely by registering an
// actual Asynq server, which is exactly what the worker does and exactly what
// the worker_absent check reads.
package monitor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func TestSmokeDiscordAlerting(t *testing.T) {
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	redisURL := os.Getenv("BODHVEDA_REDIS_URL")
	webhookURL := os.Getenv("BODHVEDA_ALERT_DISCORD_WEBHOOK_URL")

	if redisURL == "" {
		t.Fatal("BODHVEDA_REDIS_URL is empty")
	}
	if webhookURL == "" {
		t.Fatal("BODHVEDA_ALERT_DISCORD_WEBHOOK_URL is empty — nothing to smoke test")
	}
	t.Logf("redis configured, discord webhook configured (%d chars)", len(webhookURL))

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatalf("parse redis uri: %v", err)
	}

	inspector := asynq.NewInspector(opt)
	defer inspector.Close()

	sink := NewDiscordSink(webhookURL)
	if sink == nil {
		t.Fatal("expected a non-nil sink for a configured webhook URL")
	}

	m := New(Config{
		Checks: DefaultChecks(inspector, nil, nil),
		Sink:   sink,
	})

	// --- Phase 1: the condition is genuinely true. ---------------------------
	servers, err := inspector.Servers()
	if err != nil {
		t.Fatalf("inspect servers: %v", err)
	}
	t.Logf("phase 1: %d asynq server(s) registered", len(servers))
	if len(servers) != 0 {
		t.Skip("a worker is already running, so worker_absent cannot fire; stop the worker and re-run")
	}

	// worker_absent has MinConsecutive=2, so it opens on the second tick. That is
	// the real configured behaviour, not a shortcut.
	m.Tick(context.Background())
	if m.tracker.IsOpen("worker_absent") {
		t.Fatal("worker_absent must not open on a single tick (MinConsecutive=2)")
	}
	t.Log("phase 1: tick 1 — condition observed, not yet open (as designed)")

	m.Tick(context.Background())
	if !m.tracker.IsOpen("worker_absent") {
		t.Fatal("worker_absent should have opened on the second consecutive tick")
	}
	t.Log("phase 1: tick 2 — condition OPENED, red alert posted to Discord")

	// --- Phase 2: resolve it for real. ---------------------------------------
	// Registering a real Asynq server is precisely what cmd/worker does, and
	// precisely what Inspector.Servers() reads — so this is a genuine recovery,
	// not a simulated one.
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 1})
	mux := asynq.NewServeMux()

	go func() {
		if err := srv.Run(mux); err != nil {
			t.Logf("asynq server exited: %v", err)
		}
	}()
	defer srv.Shutdown()

	// Give the server a moment to write its first heartbeat into Redis.
	deadline := time.Now().Add(30 * time.Second)
	for {
		servers, err = inspector.Servers()
		if err == nil && len(servers) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("asynq server never registered in Redis")
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Logf("phase 2: %d asynq server(s) now registered", len(servers))

	m.Tick(context.Background())
	if m.tracker.IsOpen("worker_absent") {
		t.Fatal("worker_absent should have resolved once a server registered")
	}
	t.Log("phase 2: condition RESOLVED, green alert posted to Discord")

	t.Log("smoke test complete — expect exactly 2 Discord messages: red 'Firing: worker absent', then green 'Resolved: worker absent'")
}
