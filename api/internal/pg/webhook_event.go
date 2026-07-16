package pg

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type WebhookEventRepo struct {
	db dbx.DBExecutor
}

func NewWebhookEventRepo(db *pgxpool.Pool) repository.WebhookEventRepository {
	return &WebhookEventRepo{db: db}
}

// Claim inserts the event id, treating a unique-conflict as "already seen". The
// ON CONFLICT DO NOTHING makes it atomic: exactly one concurrent inserter gets
// RowsAffected() == 1 (proceed), the rest get 0 (duplicate).
func (r *WebhookEventRepo) Claim(ctx context.Context, projectID int, provider, providerEventID string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO webhook_event (project_id, provider, provider_event_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, provider_event_id) DO NOTHING
	`, projectID, provider, providerEventID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *WebhookEventRepo) Release(ctx context.Context, provider, providerEventID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM webhook_event WHERE provider = $1 AND provider_event_id = $2
	`, provider, providerEventID)
	return err
}

func (r *WebhookEventRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM webhook_event WHERE received_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
