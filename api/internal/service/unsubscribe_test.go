package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/email"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/pg"
)

// TestUnsubscribeService_EndToEnd verifies the Phase 6 loop against a real
// Postgres: a token built the way fanOutEmail builds it, when redeemed, flips the
// recipient's email preference off so ShouldDirectNotificationBeDelivered(email)
// goes true → false, and a repeat unsubscribe is idempotent. Skipped unless
// TEST_DB_URL is set; uses scratch project id 1 and cleans up. (The thin public
// handler is covered by handler-package tests / driven live.)
func TestUnsubscribeService_EndToEnd(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}

	env.HashKey = "unsub-test-hash-key-0123456789ab"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	projectID := 1
	const recipientExtID = "unsub-test-recipient"
	target := dto.Target{Channel: "digest", Topic: "none", Event: "sent"}

	preferenceRepo := pg.NewPreferenceRepo(pool)
	preferenceService := NewProjectPreferenceService(preferenceRepo, pg.NewRecipientRepo(pool))
	svc := NewUnsubscribeService(preferenceService)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM preference WHERE project_id = $1 AND (recipient_external_id = $2 OR recipient_external_id IS NULL AND channel = 'digest' AND topic = 'none' AND event = 'sent')", projectID, recipientExtID)
	})

	// Catalog the (target, email) medium at the project level so email is eligible
	// to deliver before the unsubscribe (non-in_app defaults to NOT deliver unless
	// cataloged — the catalog gate).
	name := "Digest email"
	catalog := entity.NewPreference(&projectID, nil, target.Channel, target.Topic, target.Event, string(enum.MediumEmail), &name, nil, true)
	if _, err := preferenceRepo.Create(ctx, catalog); err != nil {
		t.Fatalf("seed catalog preference: %v", err)
	}

	// Before: email should deliver (cataloged, no recipient opt-out).
	deliver, err := preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, projectID, recipientExtID, target, enum.MediumEmail)
	if err != nil {
		t.Fatalf("gate before: %v", err)
	}
	if !deliver {
		t.Fatalf("email should deliver before unsubscribe, got false")
	}

	// Redeem a token exactly as fanOutEmail builds it.
	token, err := email.BuildUnsubscribeToken(email.UnsubscribeClaims{
		ProjectID: projectID, RecipientExtID: recipientExtID,
		Channel: target.Channel, Topic: target.Topic, Event: target.Event,
	}, []byte(env.HashKey))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	gotTarget, errKind, err := svc.UnsubscribeEmail(ctx, token)
	if err != nil {
		t.Fatalf("unsubscribe: errKind=%v err=%v", errKind, err)
	}
	if gotTarget != target {
		t.Errorf("returned target = %+v, want %+v", gotTarget, target)
	}

	// After: email must NOT deliver (recipient-level email pref now disabled).
	deliver, err = preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, projectID, recipientExtID, target, enum.MediumEmail)
	if err != nil {
		t.Fatalf("gate after: %v", err)
	}
	if deliver {
		t.Errorf("email should be muted after unsubscribe, got deliver=true")
	}

	// Idempotent: a second unsubscribe still succeeds.
	if _, _, err := svc.UnsubscribeEmail(ctx, token); err != nil {
		t.Errorf("second unsubscribe should be idempotent, got %v", err)
	}

	// A malformed token is rejected as invalid input (→ 400 at the handler).
	if _, errKind, err := svc.UnsubscribeEmail(ctx, "not-a-real-token"); err == nil {
		t.Errorf("malformed token should error, got errKind=%v", errKind)
	}
}
