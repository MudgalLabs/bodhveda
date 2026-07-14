package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type NotificationDeliveryRepo struct {
	db dbx.DBExecutor
}

func NewNotificationDeliveryRepo(db *pgxpool.Pool) repository.NotificationDeliveryRepository {
	return &NotificationDeliveryRepo{
		db: db,
	}
}

const notificationDeliveryFields = `
	id, notification_id, project_id, recipient_external_id, medium, contact_id, address_snapshot,
	status, provider, provider_message_id, failure_reason, attempt, sent_at, created_at, updated_at
`

func scanNotificationDelivery(row interface {
	Scan(dest ...any) error
}) (*entity.NotificationDelivery, error) {
	var d entity.NotificationDelivery
	var medium, status string
	err := row.Scan(
		&d.ID, &d.NotificationID, &d.ProjectID, &d.RecipientExtID, &medium, &d.ContactID, &d.AddressSnapshot,
		&status, &d.Provider, &d.ProviderMessageID, &d.FailureReason, &d.Attempt, &d.SentAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Medium = enum.Medium(medium)
	d.Status = enum.DeliveryStatus(status)
	return &d, nil
}

func (r *NotificationDeliveryRepo) Create(ctx context.Context, delivery *entity.NotificationDelivery) (*entity.NotificationDelivery, error) {
	sql := fmt.Sprintf(`
		INSERT INTO notification_delivery
			(notification_id, project_id, recipient_external_id, medium, contact_id, address_snapshot,
			 status, provider, provider_message_id, failure_reason, attempt, sent_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING %s
	`, notificationDeliveryFields)

	row := r.db.QueryRow(ctx, sql,
		delivery.NotificationID, delivery.ProjectID, delivery.RecipientExtID, string(delivery.Medium),
		delivery.ContactID, delivery.AddressSnapshot, string(delivery.Status), delivery.Provider,
		delivery.ProviderMessageID, delivery.FailureReason, delivery.Attempt, delivery.SentAt,
		delivery.CreatedAt, delivery.UpdatedAt,
	)

	created, err := scanNotificationDelivery(row)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return created, nil
}

func (r *NotificationDeliveryRepo) Get(ctx context.Context, id int64) (*entity.NotificationDelivery, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM notification_delivery
		WHERE id = $1
	`, notificationDeliveryFields)

	row := r.db.QueryRow(ctx, sql, id)
	delivery, err := scanNotificationDelivery(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return delivery, nil
}

func (r *NotificationDeliveryRepo) ListForNotification(ctx context.Context, notificationID int) ([]*entity.NotificationDelivery, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM notification_delivery
		WHERE notification_id = $1
		ORDER BY medium ASC, id ASC
	`, notificationDeliveryFields)

	rows, err := r.db.Query(ctx, sql, notificationID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	deliveries := []*entity.NotificationDelivery{}
	for rows.Next() {
		delivery, err := scanNotificationDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return deliveries, nil
}

func (r *NotificationDeliveryRepo) UpdateResult(ctx context.Context, id int64, result repository.NotificationDeliveryResult) error {
	now := time.Now().UTC()

	// sent_at is stamped only on a successful send.
	var sentAt *time.Time
	if result.Status == enum.DeliverySent {
		sentAt = &now
	}

	sql := `
		UPDATE notification_delivery
		SET status = $1,
		    provider = $2,
		    provider_message_id = $3,
		    failure_reason = $4,
		    attempt = $5,
		    sent_at = COALESCE($6, sent_at),
		    updated_at = $7
		WHERE id = $8
	`

	res, err := r.db.Exec(ctx, sql,
		string(result.Status), result.Provider, result.ProviderMessageID, result.FailureReason,
		result.Attempt, sentAt, now, id,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return nil
}

// deliveryStatusRank maps a status onto a monotonic rank. A webhook status is
// applied only when its rank is strictly greater than the row's current rank,
// which makes transitions order-tolerant and non-regressing: a late `delivered`
// (3) never overwrites a `bounced`/`complained`/`failed` (4), and the first
// terminal among {bounced, complained, failed} wins (equal rank ⇒ not overwritten).
// `%s` is substituted with either `status` (the current column) or the incoming
// status literal so the same ladder ranks both.
const deliveryStatusRank = `(CASE %s
	WHEN 'pending' THEN 0
	WHEN 'sending' THEN 1
	WHEN 'sent' THEN 2
	WHEN 'delivered' THEN 3
	WHEN 'bounced' THEN 4
	WHEN 'complained' THEN 4
	WHEN 'failed' THEN 4
	ELSE 5 END)`

func (r *NotificationDeliveryRepo) ApplyWebhookStatus(ctx context.Context, u repository.DeliveryWebhookUpdate) error {
	var newStatus *string
	if u.Status != nil {
		s := string(*u.Status)
		newStatus = &s
	}

	// $1 provider_message_id, $2 new status (nullable), $3 event kind, $4 event
	// time, $5 raw event JSON. The status only advances when $2 outranks the
	// current status; each *_at column is first-write-wins (COALESCE); the raw
	// event is appended to the provider_response JSONB array.
	sql := fmt.Sprintf(`
		UPDATE notification_delivery SET
			status = CASE
				WHEN $2::text IS NULL THEN status
				WHEN %s > %s THEN $2::text
				ELSE status END,
			delivered_at  = CASE WHEN $3 = 'delivered'  THEN COALESCE(delivered_at,  $4) ELSE delivered_at  END,
			bounced_at    = CASE WHEN $3 = 'bounced'    THEN COALESCE(bounced_at,    $4) ELSE bounced_at    END,
			complained_at = CASE WHEN $3 = 'complained' THEN COALESCE(complained_at, $4) ELSE complained_at END,
			opened_at     = CASE WHEN $3 = 'opened'     THEN COALESCE(opened_at,     $4) ELSE opened_at     END,
			clicked_at    = CASE WHEN $3 = 'clicked'    THEN COALESCE(clicked_at,    $4) ELSE clicked_at    END,
			provider_response = COALESCE(provider_response, '[]'::jsonb) || $5::jsonb,
			updated_at = now()
		WHERE provider_message_id = $1
	`, fmt.Sprintf(deliveryStatusRank, "$2::text"), fmt.Sprintf(deliveryStatusRank, "status"))

	res, err := r.db.Exec(ctx, sql, u.ProviderMessageID, newStatus, u.Kind, u.At, string(u.RawEvent))
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return nil
}

func (r *NotificationDeliveryRepo) GetTargetByProviderMessageID(ctx context.Context, providerMessageID string) (*repository.DeliveryTarget, error) {
	sql := `
		SELECT nd.project_id, nd.recipient_external_id, n.channel, n.topic, n.event
		FROM notification_delivery nd
		JOIN notification n ON n.id = nd.notification_id
		WHERE nd.provider_message_id = $1
	`
	var t repository.DeliveryTarget
	err := r.db.QueryRow(ctx, sql, providerMessageID).Scan(
		&t.ProjectID, &t.RecipientExtID, &t.Channel, &t.Topic, &t.Event,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *NotificationDeliveryRepo) EmailDeliveryOverviewForProject(ctx context.Context, projectID int) (*dto.EmailDeliveryOverview, error) {
	sql := `
		SELECT
			count(*),
			count(*) FILTER (WHERE status = 'pending'),
			count(*) FILTER (WHERE status = 'sent'),
			count(*) FILTER (WHERE status = 'delivered'),
			count(*) FILTER (WHERE status = 'bounced'),
			count(*) FILTER (WHERE status = 'complained'),
			count(*) FILTER (WHERE status = 'failed'),
			count(*) FILTER (WHERE status = 'no_contact'),
			count(*) FILTER (WHERE status = 'muted'),
			count(*) FILTER (WHERE opened_at IS NOT NULL),
			count(*) FILTER (WHERE clicked_at IS NOT NULL)
		FROM notification_delivery
		WHERE project_id = $1 AND medium = 'email'
	`

	var o dto.EmailDeliveryOverview
	err := r.db.QueryRow(ctx, sql, projectID).Scan(
		&o.Total, &o.Pending, &o.Sent, &o.Delivered, &o.Bounced, &o.Complained,
		&o.Failed, &o.NoContact, &o.Muted, &o.Opened, &o.Clicked,
	)
	if err != nil {
		return nil, err
	}

	return &o, nil
}
