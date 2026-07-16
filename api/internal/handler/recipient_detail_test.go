package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	// chi v1 — the version cmd/api/routes.go and tantra's httpx.ParamInt use.
	// Mounting on chi/v5 here would silently fail to resolve URL params, since
	// each version keeps its route context under its own key.
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
)

// TestGetRecipientConsoleHandler drives Phase 9.2's single-recipient console read
// over real HTTP through a chi router mounted exactly as cmd/api/routes.go mounts
// it. The service test covers the query and the project scoping; this covers what
// only the HTTP layer can break — that a customer-chosen external_id survives the
// round trip through a URL path segment.
//
// The auth middleware is intentionally NOT mounted: it is orthogonal (and already
// gates the live route).
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestGetRecipientConsoleHandler(t *testing.T) {
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

	// Deliberately awkward external ids. These are customer-chosen strings, and
	// nothing stops a customer from using an email address or a path-like id.
	plainExtID := "rdh-plain-user"
	emailExtID := "rdh+tag@example.com"

	for _, extID := range []string{plainExtID, emailExtID} {
		_, err = pool.Exec(ctx, `
			INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
			VALUES ($1, 'RDH Test', $2, now(), now())
		`, extID, projectID)
		if err != nil {
			t.Fatalf("insert recipient %q: %v", extID, err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, "DELETE FROM recipient WHERE project_id = $1 AND external_id = $2",
				projectID, extID)
		})
	}

	svc := service.NewRecipientService(pg.NewRecipientRepo(pool), nil)

	r := chi.NewRouter()
	r.Route("/console/projects/{project_id}", func(r chi.Router) {
		r.Route("/recipients", func(r chi.Router) {
			r.Get("/{recipient_external_id}", GetRecipientConsole(svc))
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	get := func(path string) (*http.Response, []byte) {
		t.Helper()
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res, body
	}

	type recipientPayload struct {
		Data struct {
			ID                          string `json:"id"`
			Name                        string `json:"name"`
			DirectNotificationsCount    int    `json:"direct_notifications_count"`
			BroadcastNotificationsCount int    `json:"broadcast_notifications_count"`
		} `json:"data"`
	}

	t.Run("returns the recipient with its counts", func(t *testing.T) {
		res, body := get("/console/projects/1/recipients/" + plainExtID)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200. body: %s", res.StatusCode, body)
		}

		var parsed recipientPayload
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode: %v. body: %s", err, body)
		}
		if parsed.Data.ID != plainExtID {
			t.Errorf("id = %q, want %q", parsed.Data.ID, plainExtID)
		}
		// The console renders these on the Overview tab; a recipient with no
		// sends must report zero rather than omit them.
		if parsed.Data.DirectNotificationsCount != 0 || parsed.Data.BroadcastNotificationsCount != 0 {
			t.Errorf("counts = %d/%d, want 0/0",
				parsed.Data.DirectNotificationsCount, parsed.Data.BroadcastNotificationsCount)
		}
	})

	// The console percent-encodes the id (API_ROUTES) and the router encodes the
	// route param. This proves the encoded form actually resolves server-side.
	t.Run("an email-shaped external id survives the URL round trip", func(t *testing.T) {
		res, body := get("/console/projects/1/recipients/" + url.PathEscape(emailExtID))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200. body: %s", res.StatusCode, body)
		}

		var parsed recipientPayload
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode: %v. body: %s", err, body)
		}
		if parsed.Data.ID != emailExtID {
			t.Errorf("id = %q, want %q — the id did not survive encoding", parsed.Data.ID, emailExtID)
		}
	})

	t.Run("an unknown recipient is a 404", func(t *testing.T) {
		res, _ := get("/console/projects/1/recipients/rdh-does-not-exist")
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", res.StatusCode)
		}
	})

	t.Run("rejects a non-numeric project id", func(t *testing.T) {
		res, _ := get("/console/projects/not-a-number/recipients/" + plainExtID)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", res.StatusCode)
		}
	})
}
