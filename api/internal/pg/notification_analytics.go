package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
)

// InAppAnalyticsSeries returns per-day in-app notification counts for a project
// over the date range, bucketed by DAY in the viewer's timezone (`tz`, an IANA
// name — the caller validated it via time.LoadLocation). It aggregates the
// `notification` row's own `status` scalar (the in-app inbox outcome) and never
// touches notification_delivery — see dto.ProjectAnalytics for why the two sides
// are separate aggregates rather than one join.
//
// Only days with at least one notification are returned; the console gap-fills
// zeros across the range. `from`/`to` are inclusive instants; nil means unbounded
// on that side.
//
// The leading `project_id = $1` predicate rides ix_notification_project_id
// (project_id, id DESC) to reach one project's rows; from there this is an
// aggregate, so it must read every matching row (there is no partial-aggregate
// shortcut). Measured acceptable on 200k+ rows — see the Phase 9.5 deviations.
func (r *NotificationRepo) InAppAnalyticsSeries(ctx context.Context, projectID int, from, to *time.Time, tz string) ([]dto.AnalyticsInAppDay, error) {
	// $4 (tz) is bound as a parameter to AT TIME ZONE, which accepts a text
	// expression — so the wall-clock day boundaries are the viewer's, not UTC's.
	sql := `
		SELECT
			to_char(date_trunc('day', created_at AT TIME ZONE $4), 'YYYY-MM-DD') AS day,
			count(*) AS total,
			count(*) FILTER (WHERE status = 'enqueued') AS enqueued,
			count(*) FILTER (WHERE status = 'muted') AS muted,
			count(*) FILTER (WHERE status = 'delivered') AS delivered,
			count(*) FILTER (WHERE status = 'quota_exceeded') AS quota_exceeded,
			count(*) FILTER (WHERE status = 'failed') AS failed
		FROM notification
		WHERE project_id = $1
			AND ($2::timestamptz IS NULL OR created_at >= $2)
			AND ($3::timestamptz IS NULL OR created_at <= $3)
		GROUP BY day
		ORDER BY day
	`

	rows, err := r.db.Query(ctx, sql, projectID, from, to, tz)
	if err != nil {
		return nil, fmt.Errorf("in-app analytics series query: %w", err)
	}
	defer rows.Close()

	series := []dto.AnalyticsInAppDay{}
	for rows.Next() {
		var d dto.AnalyticsInAppDay
		if err := rows.Scan(&d.Day, &d.Total, &d.Enqueued, &d.Muted, &d.Delivered,
			&d.QuotaExceeded, &d.Failed); err != nil {
			return nil, fmt.Errorf("scan in-app analytics day: %w", err)
		}
		series = append(series, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return series, nil
}

// TargetVolumes returns each {channel, topic, event} target's in-app notification
// count over the range, most-active first. It answers "which targets actually
// fire" from the `notification` table alone; the service merges the per-target
// email stats (from the delivery repo) on top.
//
// Capped at `limit` targets: a breakdown chart shows the top handful, and an
// unbounded list on a project with thousands of distinct targets would be a
// payload nobody reads.
func (r *NotificationRepo) TargetVolumes(ctx context.Context, projectID int, from, to *time.Time, limit int) ([]dto.AnalyticsTargetStat, error) {
	sql := `
		SELECT channel, topic, event, count(*) AS notifications
		FROM notification
		WHERE project_id = $1
			AND ($2::timestamptz IS NULL OR created_at >= $2)
			AND ($3::timestamptz IS NULL OR created_at <= $3)
		GROUP BY channel, topic, event
		ORDER BY notifications DESC, channel, topic, event
		LIMIT $4
	`

	rows, err := r.db.Query(ctx, sql, projectID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("target volumes query: %w", err)
	}
	defer rows.Close()

	targets := []dto.AnalyticsTargetStat{}
	for rows.Next() {
		var t dto.AnalyticsTargetStat
		if err := rows.Scan(&t.Channel, &t.Topic, &t.Event, &t.Notifications); err != nil {
			return nil, fmt.Errorf("scan target volume: %w", err)
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return targets, nil
}
