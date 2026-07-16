package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
	tantraService "github.com/mudgallabs/tantra/service"
)

// TestEmailWebhookService_Ingest drives the full Phase 5 webhook path end to end
// against a real Postgres: it seeds an email delivery row, signs a Resend/Svix
// webhook the way Resend would, and asserts the row transitions (and that a bad
// signature is rejected). Skipped unless TEST_DB_URL is set. Uses scratch project
// id 1 (same convention as the other pg/service integration tests) and cleans up.
func TestEmailWebhookService_Ingest(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	env.CipherKey = "0123456789abcdef0123456789abcdef"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const projectID = 1
	const recipientExtID = "wh-test-recipient"
	const providerMessageID = "wh_test_msg_1"

	settingsRepo := pg.NewProjectEmailSettingsRepo(pool)
	deliveryRepo := pg.NewNotificationDeliveryRepo(pool)
	webhookEventRepo := pg.NewWebhookEventRepo(pool)
	preferenceRepo := pg.NewPreferenceRepo(pool)
	preferenceService := NewProjectPreferenceService(preferenceRepo, pg.NewRecipientRepo(pool))
	svc := NewEmailWebhookService(settingsRepo, deliveryRepo, webhookEventRepo, preferenceService)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM webhook_event WHERE project_id = $1", projectID)
	})

	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("webhook-signing-key-material"))

	// --- Seed: email settings (with webhook secret), a notification, a sent delivery row.
	settings, err := entity.NewProjectEmailSettings(projectID, enum.EmailProviderResend, "re_key", "Acme", "hey@acme.com")
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}
	if err := settings.SetWebhookSecret(secret); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}
	if _, err := settingsRepo.Upsert(ctx, settings); err != nil {
		t.Fatalf("upsert settings: %v", err)
	}

	var notificationID int
	err = pool.QueryRow(ctx, `
		INSERT INTO notification (project_id, recipient_external_id, payload, channel, topic, event, status, created_at, updated_at)
		VALUES ($1, $2, '{}'::jsonb, 'test', 'none', 'wh', 'delivered', now(), now())
		RETURNING id
	`, projectID, recipientExtID).Scan(&notificationID)
	if err != nil {
		t.Fatalf("insert notification: %v", err)
	}

	var deliveryID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, status, provider, provider_message_id, attempt, created_at, updated_at)
		VALUES ($1, $2, $3, 'email', 'sent', 'resend', $4, 0, now(), now())
		RETURNING id
	`, notificationID, projectID, recipientExtID, providerMessageID).Scan(&deliveryID)
	if err != nil {
		t.Fatalf("insert delivery: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM notification_delivery WHERE id = $1", deliveryID)
		_, _ = pool.Exec(ctx, "DELETE FROM notification WHERE id = $1", notificationID)
		_, _ = pool.Exec(ctx, "DELETE FROM project_email_settings WHERE project_id = $1", projectID)
	})

	body := []byte(fmt.Sprintf(
		`{"type":"email.delivered","created_at":"2026-07-14T10:00:00Z","data":{"email_id":%q}}`,
		providerMessageID,
	))

	// --- Bad signature → 401 (auth is the signature).
	badHeaders := signTestSvix(t, secret, "msg_bad", time.Now().Unix(), []byte(`{"tampered":true}`))
	if errKind, err := svc.Ingest(ctx, projectID, badHeaders, body); errKind != tantraService.ErrUnauthorized {
		t.Fatalf("bad signature: got errKind=%v err=%v, want ErrUnauthorized", errKind, err)
	}

	// --- Valid signature → row transitions sent → delivered, delivered_at stamped.
	goodHeaders := signTestSvix(t, secret, "msg_good", time.Now().Unix(), body)
	if errKind, err := svc.Ingest(ctx, projectID, goodHeaders, body); errKind != tantraService.ErrNone || err != nil {
		t.Fatalf("valid webhook: errKind=%v err=%v", errKind, err)
	}

	var status string
	var deliveredAt *time.Time
	var respLen int
	err = pool.QueryRow(ctx, `
		SELECT status, delivered_at, jsonb_array_length(COALESCE(provider_response, '[]'::jsonb))
		FROM notification_delivery WHERE id = $1
	`, deliveryID).Scan(&status, &deliveredAt, &respLen)
	if err != nil {
		t.Fatalf("read back delivery: %v", err)
	}
	if status != "delivered" {
		t.Errorf("status = %q, want delivered", status)
	}
	if deliveredAt == nil {
		t.Errorf("delivered_at not stamped")
	}
	if respLen != 1 {
		t.Errorf("provider_response length = %d, want 1", respLen)
	}

	// --- Idempotency (#8): re-delivering the SAME event (same svix-id) is a no-op.
	// It must ack without re-appending to provider_response.
	if errKind, err := svc.Ingest(ctx, projectID, goodHeaders, body); errKind != tantraService.ErrNone || err != nil {
		t.Fatalf("duplicate webhook: errKind=%v err=%v, want acked no-op", errKind, err)
	}
	var respLenAfterDup int
	_ = pool.QueryRow(ctx, `
		SELECT jsonb_array_length(COALESCE(provider_response, '[]'::jsonb))
		FROM notification_delivery WHERE id = $1
	`, deliveryID).Scan(&respLenAfterDup)
	if respLenAfterDup != 1 {
		t.Errorf("after duplicate: provider_response length = %d, want still 1 (deduped)", respLenAfterDup)
	}

	// --- Non-regression: a late "sent" event must NOT overwrite delivered.
	lateBody := []byte(fmt.Sprintf(`{"type":"email.sent","created_at":"2026-07-14T10:05:00Z","data":{"email_id":%q}}`, providerMessageID))
	lateHeaders := signTestSvix(t, secret, "msg_late", time.Now().Unix(), lateBody)
	if _, err := svc.Ingest(ctx, projectID, lateHeaders, lateBody); err != nil {
		t.Fatalf("late sent event: %v", err)
	}
	_ = pool.QueryRow(ctx, "SELECT status FROM notification_delivery WHERE id = $1", deliveryID).Scan(&status)
	if status != "delivered" {
		t.Errorf("after late sent: status = %q, want still delivered", status)
	}

	// --- Complaint suppression (Phase 6): a `complained` event flips the email
	// medium preference for (recipient, target) off, so subsequent sends are muted.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM preference WHERE project_id = $1 AND recipient_external_id = $2", projectID, recipientExtID)
	})
	complaintBody := []byte(fmt.Sprintf(`{"type":"email.complained","created_at":"2026-07-14T10:10:00Z","data":{"email_id":%q}}`, providerMessageID))
	complaintHeaders := signTestSvix(t, secret, "msg_complaint", time.Now().Unix(), complaintBody)
	if errKind, err := svc.Ingest(ctx, projectID, complaintHeaders, complaintBody); errKind != tantraService.ErrNone || err != nil {
		t.Fatalf("complaint webhook: errKind=%v err=%v", errKind, err)
	}
	var prefEnabled bool
	err = pool.QueryRow(ctx, `
		SELECT enabled FROM preference
		WHERE project_id = $1 AND recipient_external_id = $2 AND channel = 'test' AND topic = 'none' AND event = 'wh' AND medium = 'email'
	`, projectID, recipientExtID).Scan(&prefEnabled)
	if err != nil {
		t.Fatalf("read back email preference after complaint: %v", err)
	}
	if prefEnabled {
		t.Errorf("email preference still enabled after complaint; want disabled (suppressed)")
	}
}

func signTestSvix(t *testing.T, secret, msgID string, ts int64, body []byte) http.Header {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(secret[len("whsec_"):])
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	tsStr := strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID + "." + tsStr + "."))
	mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	h := http.Header{}
	h.Set("svix-id", msgID)
	h.Set("svix-timestamp", tsStr)
	h.Set("svix-signature", "v1,"+sig)
	return h
}
