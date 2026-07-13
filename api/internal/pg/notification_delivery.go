package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
