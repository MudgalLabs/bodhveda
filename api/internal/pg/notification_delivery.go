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
	status, provider, provider_message_id, failure_reason, attempt, sent_at, delivered_at, bounced_at,
	complained_at, opened_at, clicked_at, provider_response, created_at, updated_at
`

func scanNotificationDelivery(row interface {
	Scan(dest ...any) error
}) (*entity.NotificationDelivery, error) {
	var d entity.NotificationDelivery
	var medium, status string
	// provider_response is scanned as raw bytes rather than json.RawMessage so a
	// SQL NULL (no webhook has landed yet) stays nil instead of decoding.
	var providerResponse []byte
	err := row.Scan(
		&d.ID, &d.NotificationID, &d.ProjectID, &d.RecipientExtID, &medium, &d.ContactID, &d.AddressSnapshot,
		&status, &d.Provider, &d.ProviderMessageID, &d.FailureReason, &d.Attempt, &d.SentAt, &d.DeliveredAt,
		&d.BouncedAt, &d.ComplainedAt, &d.OpenedAt, &d.ClickedAt, &providerResponse, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Medium = enum.Medium(medium)
	d.Status = enum.DeliveryStatus(status)
	d.ProviderResponse = providerResponse
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

func (r *NotificationDeliveryRepo) ListForNotification(ctx context.Context, projectID, notificationID int) ([]*entity.NotificationDelivery, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM notification_delivery
		WHERE notification_id = $1 AND project_id = $2
		ORDER BY medium ASC, id ASC
	`, notificationDeliveryFields)

	rows, err := r.db.Query(ctx, sql, notificationID, projectID)
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
	// time, $5 raw event JSON, $6 project_id. The match is scoped to the project so
	// one project's webhook can't touch another's row. The status only advances when
	// $2 outranks the current status; each *_at column is first-write-wins
	// (COALESCE); the raw event is appended to the provider_response JSONB array.
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
		WHERE provider_message_id = $1 AND project_id = $6
	`, fmt.Sprintf(deliveryStatusRank, "$2::text"), fmt.Sprintf(deliveryStatusRank, "status"))

	res, err := r.db.Exec(ctx, sql, u.ProviderMessageID, newStatus, u.Kind, u.At, string(u.RawEvent), u.ProjectID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return nil
}

func (r *NotificationDeliveryRepo) GetTargetByProviderMessageID(ctx context.Context, projectID int, providerMessageID string) (*repository.DeliveryTarget, error) {
	sql := `
		SELECT nd.project_id, nd.recipient_external_id, n.channel, n.topic, n.event
		FROM notification_delivery nd
		JOIN notification n ON n.id = nd.notification_id
		WHERE nd.provider_message_id = $1 AND nd.project_id = $2
	`
	var t repository.DeliveryTarget
	err := r.db.QueryRow(ctx, sql, providerMessageID, projectID).Scan(
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

// EmailAnalyticsSeries returns per-day email delivery counts for a project over
// the range, bucketed by DAY in the viewer's timezone (`tz`). It aggregates
// `notification_delivery WHERE medium='email'` — the SEPARATE table where the
// email medium's outcome lives (dto.ProjectAnalytics explains why in-app and
// email are never one join). The full per-status split plus soft opened/clicked
// signals are summed by the service from these rows; only the four
// delivery-health statuses that drive the over-time chart are broken out per day.
//
// Served by ix_nd_email_status_time (project_id, created_at DESC) WHERE
// medium='email' — the partial index whose predicate matches this query exactly
// (note: despite its name it has no `status` column — the per-status FILTERs are
// counted after the index narrows the rows). Only days with ≥1 email appear;
// the console gap-fills.
func (r *NotificationDeliveryRepo) EmailAnalyticsSeries(ctx context.Context, projectID int, from, to *time.Time, tz string) ([]dto.AnalyticsEmailDay, *dto.AnalyticsEmail, error) {
	sql := `
		SELECT
			to_char(date_trunc('day', created_at AT TIME ZONE $4), 'YYYY-MM-DD') AS day,
			count(*) AS attempted,
			count(*) FILTER (WHERE status = 'pending') AS pending,
			count(*) FILTER (WHERE status = 'sent') AS sent,
			count(*) FILTER (WHERE status = 'delivered') AS delivered,
			count(*) FILTER (WHERE status = 'bounced') AS bounced,
			count(*) FILTER (WHERE status = 'complained') AS complained,
			count(*) FILTER (WHERE status = 'failed') AS failed,
			count(*) FILTER (WHERE status = 'no_contact') AS no_contact,
			count(*) FILTER (WHERE status = 'muted') AS muted,
			count(*) FILTER (WHERE opened_at IS NOT NULL) AS opened,
			count(*) FILTER (WHERE clicked_at IS NOT NULL) AS clicked
		FROM notification_delivery
		WHERE project_id = $1 AND medium = 'email'
			AND ($2::timestamptz IS NULL OR created_at >= $2)
			AND ($3::timestamptz IS NULL OR created_at <= $3)
		GROUP BY day
		ORDER BY day
	`

	rows, err := r.db.Query(ctx, sql, projectID, from, to, tz)
	if err != nil {
		return nil, nil, fmt.Errorf("email analytics series query: %w", err)
	}
	defer rows.Close()

	series := []dto.AnalyticsEmailDay{}
	totals := &dto.AnalyticsEmail{}
	for rows.Next() {
		var (
			day                                                      string
			attempted, pending, sent, delivered, bounced, complained int
			failed, noContact, muted, opened, clicked                int
		)
		if err := rows.Scan(&day, &attempted, &pending, &sent, &delivered, &bounced,
			&complained, &failed, &noContact, &muted, &opened, &clicked); err != nil {
			return nil, nil, fmt.Errorf("scan email analytics day: %w", err)
		}

		series = append(series, dto.AnalyticsEmailDay{
			Day: day, Attempted: attempted, Delivered: delivered,
			Bounced: bounced, Complained: complained,
		})

		// Sum the per-status split from the same scan — the series is bounded by
		// the number of days in range, so a second aggregate query for totals
		// would be a wasted scan.
		totals.Attempted += attempted
		totals.ByStatus.Pending += pending
		totals.ByStatus.Sent += sent
		totals.ByStatus.Delivered += delivered
		totals.ByStatus.Bounced += bounced
		totals.ByStatus.Complained += complained
		totals.ByStatus.Failed += failed
		totals.ByStatus.NoContact += noContact
		totals.ByStatus.Muted += muted
		totals.Opened += opened
		totals.Clicked += clicked
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	totals.Series = series
	return series, totals, nil
}

// EmailTargetStats returns per-target email delivery counts over the range,
// joining each email delivery row back to its notification's {channel, topic,
// event}. This join is CORRECT (unlike the in-app aggregate's would-be join):
// the question is specifically about email deliveries and the targets they fired
// on, so restricting to rows that HAVE a delivery is the intent, not a bug.
//
// Keyed the same way as TargetVolumes so the service can merge the two into one
// per-target row. Not limited here — the service caps the merged, notification-
// ranked list.
func (r *NotificationDeliveryRepo) EmailTargetStats(ctx context.Context, projectID int, from, to *time.Time) ([]dto.AnalyticsTargetStat, error) {
	sql := `
		SELECT n.channel, n.topic, n.event,
			count(*) AS attempted,
			count(*) FILTER (WHERE nd.status = 'delivered') AS delivered,
			count(*) FILTER (WHERE nd.status = 'bounced') AS bounced,
			count(*) FILTER (WHERE nd.status = 'complained') AS complained
		FROM notification_delivery nd
		JOIN notification n ON n.id = nd.notification_id
		WHERE nd.project_id = $1 AND nd.medium = 'email'
			AND ($2::timestamptz IS NULL OR nd.created_at >= $2)
			AND ($3::timestamptz IS NULL OR nd.created_at <= $3)
		GROUP BY n.channel, n.topic, n.event
	`

	rows, err := r.db.Query(ctx, sql, projectID, from, to)
	if err != nil {
		return nil, fmt.Errorf("email target stats query: %w", err)
	}
	defer rows.Close()

	stats := []dto.AnalyticsTargetStat{}
	for rows.Next() {
		var t dto.AnalyticsTargetStat
		if err := rows.Scan(&t.Channel, &t.Topic, &t.Event, &t.EmailAttempted,
			&t.EmailDelivered, &t.EmailBounced, &t.EmailComplained); err != nil {
			return nil, fmt.Errorf("scan email target stat: %w", err)
		}
		stats = append(stats, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}
