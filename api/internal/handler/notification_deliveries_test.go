package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	// chi v1 — the version cmd/api/routes.go and tantra's httpx.ParamInt use.
	// Mounting on chi/v5 here would silently fail to resolve URL params, since
	// each version keeps its route context under its own key.
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/pg"
	"github.com/mudgallabs/bodhveda/internal/service"
)

// TestListNotificationDeliveriesHandler drives the Phase 9.1 deliveries endpoint
// over real HTTP through a chi router mounted exactly as cmd/api/routes.go mounts
// it, against a real Postgres. The service-level test covers the query and the
// event normalization; this covers what only the HTTP layer can break — that the
// route's {project_id}/{notification_id} params actually reach the handler, and
// that the response serializes as the console expects.
//
// The auth middleware is intentionally NOT mounted here: it is orthogonal (and
// already gates the live route — an unauthenticated request 401s).
//
// Skipped unless TEST_DB_URL is set. Self-cleaning.
func TestListNotificationDeliveriesHandler(t *testing.T) {
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
	const recipientExtID = "dd-handler-recipient"

	var notificationID int
	err = pool.QueryRow(ctx, `
		INSERT INTO notification (project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
		VALUES ($1, $2, '{}'::jsonb, 'digest', 'none', 'handler-case', 'delivered', now(), now())
		RETURNING id
	`, projectID, recipientExtID).Scan(&notificationID)
	if err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM notification_delivery WHERE notification_id = $1", notificationID)
		_, _ = pool.Exec(ctx, "DELETE FROM notification WHERE id = $1", notificationID)
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, address_snapshot, status, failure_reason,
			 provider, provider_message_id, attempt, provider_response, created_at, updated_at)
		VALUES ($1, $2, $3, 'email', 'someone@example.com', 'bounced', 'provider_send_error', 'resend', 'dd_handler_msg', 1,
			'[{"type":"email.bounced","created_at":"2026-07-14T11:00:00.000Z","data":{"email_id":"dd_handler_msg"}}]'::jsonb,
			now(), now())
	`, notificationID, projectID, recipientExtID)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}

	// Only deliveryRepo is exercised by this endpoint; the rest of the service's
	// deps are irrelevant to it.
	svc := service.NewNotificationService(
		pg.NewNotificationRepo(pool), nil, nil, nil, nil,
		pg.NewNotificationDeliveryRepo(pool), nil, nil,
		nil, nil, nil,
	)

	// Mounted with the same nesting + param names as cmd/api/routes.go.
	r := chi.NewRouter()
	r.Route("/console/projects/{project_id}", func(r chi.Router) {
		r.Route("/notifications", func(r chi.Router) {
			r.Get("/{notification_id}/deliveries", ListNotificationDeliveries(svc))
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
		var buf [1 << 16]byte
		n, _ := res.Body.Read(buf[:])
		return res, buf[:n]
	}

	t.Run("returns the delivery with its normalized event history", func(t *testing.T) {
		res, body := get("/console/projects/1/notifications/" + strconv.Itoa(notificationID) + "/deliveries")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200. body: %s", res.StatusCode, body)
		}

		var parsed struct {
			Data struct {
				Deliveries []struct {
					Medium          string `json:"medium"`
					Status          string `json:"status"`
					FailureReason   string `json:"failure_reason"`
					AddressSnapshot string `json:"address_snapshot"`
					Attempt         int    `json:"attempt"`
					Events          []struct {
						Kind string `json:"kind"`
					} `json:"events"`
				} `json:"deliveries"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode response: %v. body: %s", err, body)
		}

		if len(parsed.Data.Deliveries) != 1 {
			t.Fatalf("got %d deliveries, want 1. body: %s", len(parsed.Data.Deliveries), body)
		}
		d := parsed.Data.Deliveries[0]
		if d.Medium != "email" || d.Status != "bounced" {
			t.Errorf("medium/status = %q/%q, want email/bounced", d.Medium, d.Status)
		}
		if d.FailureReason != "provider_send_error" {
			t.Errorf("failure_reason = %q, want provider_send_error", d.FailureReason)
		}
		if d.AddressSnapshot != "someone@example.com" {
			t.Errorf("address_snapshot = %q", d.AddressSnapshot)
		}
		if len(d.Events) != 1 || d.Events[0].Kind != "bounced" {
			t.Errorf("events = %+v, want one normalized bounced event", d.Events)
		}
	})

	t.Run("rejects a non-numeric notification id", func(t *testing.T) {
		res, _ := get("/console/projects/1/notifications/not-a-number/deliveries")
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", res.StatusCode)
		}
	})
}

