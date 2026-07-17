package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	// chi v1 — the version cmd/api/routes.go and tantra's httpx.ParamInt use.
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
)

// TestProjectAnalyticsHandler drives the Phase 9.5 analytics endpoint over real
// HTTP, mounting TimezoneMiddleware exactly as cmd/api/routes.go does. The
// service test covers the aggregation; this covers what only the HTTP layer can
// break — that the {project_id} param and the created_from/created_to query
// decode reach the handler, and (the novel part) that the X-Timezone header
// actually drives the per-day BUCKETING. A browser preflight for that header is
// why the console CORS had to allow it; here we prove the header changes output.
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestProjectAnalyticsHandler(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const projectID = 1

	// 2026-06-15 18:30 UTC == 2026-06-16 00:00 Asia/Kolkata (+05:30): the instant
	// straddles the local day boundary, so UTC and Kolkata bucket it differently.
	at := time.Date(2026, 6, 15, 18, 30, 0, 0, time.UTC)
	var notifID int
	err = pool.QueryRow(ctx, `
		INSERT INTO notification (project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
		VALUES ($1, 'an-handler-rec', '{}'::jsonb, 'digest', 'none', 'sent', 'delivered', $2, $2)
		RETURNING id
	`, projectID, at).Scan(&notifID)
	if err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM notification WHERE id = $1", notifID)
	})

	svc := service.NewNotificationService(
		pg.NewNotificationRepo(pool), nil, nil, nil, nil,
		pg.NewNotificationDeliveryRepo(pool), nil, nil,
		nil, nil, nil,
	)

	r := chi.NewRouter()
	// TimezoneMiddleware is what reads X-Timezone into the context the handler
	// passes to the service — mounted here as the root router mounts it.
	r.Use(middleware.TimezoneMiddleware)
	r.Route("/console/projects/{project_id}", func(r chi.Router) {
		r.Get("/analytics", ProjectAnalytics(svc))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// A window that contains the seed instant in both zones.
	const qs = "?created_from=2026-06-01T00:00:00Z&created_to=2026-06-30T23:59:59Z"

	dayFor := func(t *testing.T, tz string) string {
		t.Helper()
		req, _ := http.NewRequest("GET", srv.URL+"/console/projects/1/analytics"+qs, nil)
		if tz != "" {
			req.Header.Set("X-Timezone", tz)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", res.StatusCode)
		}
		var parsed struct {
			Data struct {
				InApp struct {
					Total  int `json:"total"`
					Series []struct {
						Day string `json:"day"`
					} `json:"series"`
				} `json:"in_app"`
			} `json:"data"`
		}
		if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if parsed.Data.InApp.Total != 1 || len(parsed.Data.InApp.Series) != 1 {
			t.Fatalf("expected exactly one notification on one day, got total=%d series=%d",
				parsed.Data.InApp.Total, len(parsed.Data.InApp.Series))
		}
		return parsed.Data.InApp.Series[0].Day
	}

	t.Run("X-Timezone drives the day bucket", func(t *testing.T) {
		if got := dayFor(t, "UTC"); got != "2026-06-15" {
			t.Errorf("UTC day = %q, want 2026-06-15", got)
		}
		if got := dayFor(t, "Asia/Kolkata"); got != "2026-06-16" {
			t.Errorf("Kolkata day = %q, want 2026-06-16", got)
		}
	})

	t.Run("missing X-Timezone falls back to UTC", func(t *testing.T) {
		if got := dayFor(t, ""); got != "2026-06-15" {
			t.Errorf("no-header day = %q, want 2026-06-15 (UTC fallback)", got)
		}
	})

	t.Run("inverted range is a 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET",
			srv.URL+"/console/projects/1/analytics?created_from=2026-06-30T00:00:00Z&created_to=2026-06-01T00:00:00Z", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", res.StatusCode)
		}
	})
}
