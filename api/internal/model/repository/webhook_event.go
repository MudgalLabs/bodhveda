package repository

import (
	"context"
	"time"
)

// WebhookEventRepository is the idempotency ledger for inbound provider webhooks
// (#8). It dedups on the provider's stable per-event id (e.g. Svix's `svix-id`,
// which is constant across retries of the same event) and supports retention
// cleanup of old rows.
type WebhookEventRepository interface {
	// Claim records (provider, providerEventID) as seen. It returns true when the
	// row was newly inserted (first time — the caller should process the event) and
	// false when it already existed (a provider retry/replay — the caller should
	// skip). Backed by INSERT ... ON CONFLICT DO NOTHING, so it is atomic under
	// concurrent retries.
	Claim(ctx context.Context, projectID int, provider, providerEventID string) (bool, error)
	// Release removes a claim. Used when processing did not complete and the provider
	// is being asked to retry (so the retry is not mistaken for a duplicate).
	Release(ctx context.Context, provider, providerEventID string) error
	// DeleteOlderThan removes claims received before cutoff (retention cleanup) and
	// returns how many were deleted.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
