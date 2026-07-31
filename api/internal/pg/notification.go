package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/query"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

// recipientFeedVisible is the predicate separating the RECIPIENT's inbox from
// the operator's record of everything the project sent. Every recipient-scoped
// read path shares it, because a row hidden from the feed but counted as unread
// (or flipped by mark-all-read) is a bug that only shows up as a badge counting
// things the user cannot find.
//
//   - `muted` — the recipient's own preferences rejected it.
//   - `quota_exceeded` — the project was over its plan limit; never delivered.
//   - `not_requested` — the SENDER never asked for in-app. The row exists only to
//     carry the email delivery, the analytics join, and GET /notifications/{id}.
//
// The operator's views deliberately do NOT use this — the console notifications
// list and the recipient detail panel show all three, because "why didn't they
// get it?" is answered by exactly the rows this hides. See ListNotifications.
const recipientFeedVisible = `status NOT IN ('muted', 'quota_exceeded', 'not_requested')`

type NotificationRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewNotificationRepo(db *pgxpool.Pool) repository.NotificationRepository {
	return &NotificationRepo{
		db:   db,
		pool: db,
	}
}

func (r *NotificationRepo) Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error) {
	sql := `
		INSERT INTO notification (
			project_id, recipient_external_id, payload, broadcast_id, channel,
			topic, event, read_at, opened_at, created_at, updated_at, completed_at, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event,
		read_at, opened_at, created_at, updated_at, completed_at, status
	`

	row := r.db.QueryRow(ctx, sql, notification.ProjectID, notification.RecipientExtID, notification.Payload,
		notification.BroadcastID, notification.Channel, notification.Topic, notification.Event,
		notification.ReadAt, notification.OpenedAt, notification.CreatedAt, notification.UpdatedAt,
		notification.CompletedAt, notification.Status,
	)

	var newNotification entity.Notification
	err := row.Scan(&newNotification.ID, &newNotification.ProjectID, &newNotification.RecipientExtID,
		&newNotification.Payload, &newNotification.BroadcastID, &newNotification.Channel, &newNotification.Topic,
		&newNotification.Event, &newNotification.ReadAt, &newNotification.OpenedAt, &newNotification.CreatedAt,
		&newNotification.UpdatedAt, &newNotification.CompletedAt, &newNotification.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	return &newNotification, nil
}

func (r *NotificationRepo) Get(ctx context.Context, projectID, id int) (*entity.Notification, error) {
	sql := `
		SELECT id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event,
		       read_at, opened_at, created_at, updated_at, completed_at, status
		FROM notification
		WHERE id = $1 AND project_id = $2
	`

	var n entity.Notification
	err := r.db.QueryRow(ctx, sql, id, projectID).Scan(&n.ID, &n.ProjectID, &n.RecipientExtID,
		&n.Payload, &n.BroadcastID, &n.Channel, &n.Topic, &n.Event, &n.ReadAt, &n.OpenedAt,
		&n.CreatedAt, &n.UpdatedAt, &n.CompletedAt, &n.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, fmt.Errorf("query notification: %w", err)
	}

	// Attach the email-medium delivery outcome (the reason to fetch by id at all —
	// the caller wants to know whether the email sent/delivered/bounced). Same
	// bounded projection as ListNotifications; provider_response is excluded (it is
	// served per-notification via ListForNotification). No row => no email send.
	deliverySQL := `
		SELECT status, failure_reason, attempt, provider, provider_message_id,
		       address_snapshot, sent_at, delivered_at, bounced_at, complained_at, opened_at, clicked_at
		FROM notification_delivery
		WHERE medium = 'email' AND notification_id = $1
	`
	var d entity.NotificationEmailDelivery
	derr := r.db.QueryRow(ctx, deliverySQL, n.ID).Scan(&d.Status, &d.FailureReason, &d.Attempt,
		&d.Provider, &d.ProviderMessageID, &d.AddressSnapshot, &d.SentAt, &d.DeliveredAt,
		&d.BouncedAt, &d.ComplainedAt, &d.OpenedAt, &d.ClickedAt)
	if derr == nil {
		n.Email = &d
	} else if !errors.Is(derr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query email delivery: %w", derr)
	}

	return &n, nil
}

// BatchCreateTx inserts notifications and back-fills their IDs.
//
// ⚠️ This used COPY, which is faster but returns nothing. Broadcast email needs
// the ids: a notification_delivery row hangs off notification_id, so without them
// there is no way to record an email outcome per recipient. That — not
// idempotency — is why the insert had to stop being a COPY (see
// agent-docs/overview.md §Open/next).
//
// ⚠️ IDs are matched back by recipient_external_id, NOT by the order of RETURNING.
// Postgres does not guarantee RETURNING order matches input order, and relying on
// it would misattribute every delivery row the first time the planner chose
// differently — silently mailing the right content to the wrong person's row. The
// mapping is safe because a broadcast batch holds each recipient at most once
// (its ids come from ListEligibleRecipientExtIDsForBroadcast, which is distinct
// by recipient).
func (r *NotificationRepo) BatchCreateTx(ctx context.Context, tx pgx.Tx, notifications []*entity.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	n := len(notifications)
	projectIDs := make([]int, n)
	extIDs := make([]string, n)
	payloads := make([][]byte, n)
	broadcastIDs := make([]*int, n)
	channels := make([]string, n)
	topics := make([]string, n)
	events := make([]string, n)
	readAt := make([]*time.Time, n)
	openedAt := make([]*time.Time, n)
	createdAt := make([]time.Time, n)
	updatedAt := make([]time.Time, n)
	completedAt := make([]*time.Time, n)
	statuses := make([]string, n)

	byExtID := make(map[string]*entity.Notification, n)

	for i, notification := range notifications {
		projectIDs[i] = notification.ProjectID
		extIDs[i] = notification.RecipientExtID
		payloads[i] = notification.Payload
		broadcastIDs[i] = notification.BroadcastID
		channels[i] = notification.Channel
		topics[i] = notification.Topic
		events[i] = notification.Event
		readAt[i] = notification.ReadAt
		openedAt[i] = notification.OpenedAt
		createdAt[i] = notification.CreatedAt
		updatedAt[i] = notification.UpdatedAt
		completedAt[i] = notification.CompletedAt
		statuses[i] = string(notification.Status)

		byExtID[notification.RecipientExtID] = notification
	}

	sql := `
		INSERT INTO notification (
			project_id, recipient_external_id, payload, broadcast_id,
			channel, topic, event, read_at, opened_at, created_at, updated_at, completed_at, status
		)
		SELECT * FROM unnest(
			$1::int[], $2::text[], $3::jsonb[], $4::int[],
			$5::text[], $6::text[], $7::text[], $8::timestamptz[], $9::timestamptz[],
			$10::timestamptz[], $11::timestamptz[], $12::timestamptz[], $13::text[]
		)
		RETURNING id, recipient_external_id
	`

	rows, err := tx.Query(ctx, sql,
		projectIDs, extIDs, payloads, broadcastIDs,
		channels, topics, events, readAt, openedAt, createdAt, updatedAt, completedAt, statuses,
	)
	if err != nil {
		return fmt.Errorf("insert notifications: %w", err)
	}
	defer rows.Close()

	var inserted int

	for rows.Next() {
		var id int
		var extID string

		if err := rows.Scan(&id, &extID); err != nil {
			return fmt.Errorf("scan inserted notification: %w", err)
		}

		if notification, ok := byExtID[extID]; ok {
			notification.ID = id
		}

		inserted++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read inserted notifications: %w", err)
	}

	if inserted != n {
		return fmt.Errorf("inserted %d notifications, expected %d", inserted, n)
	}

	return nil
}

func (r *NotificationRepo) Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, error) {
	sql := `
		SELECT
			(SELECT COUNT(id) FROM notification WHERE project_id = $1 AND broadcast_id IS NULL) AS direct_count,
			(SELECT COUNT(id) FROM broadcast WHERE project_id = $1) AS broadcast_count,
			(SELECT COUNT(id) FROM notification WHERE project_id = $1) AS total_count;

	`

	result := &dto.NotificationsOverviewResult{}

	err := r.db.QueryRow(ctx, sql, projectID).Scan(
		&result.TotalDirectSent,
		&result.TotalBroadcastSent,
		&result.TotalNotifications,
	)

	if err != nil {
		return nil, fmt.Errorf("overview query: %w", err)
	}

	return result, nil
}

// NOTE: this method (the Developer API's recipient inbox) has no index of its
// own. `notification` carries exactly one index besides its PK —
// ix_notification_project_id (project_id, id DESC), added in Phase 9.4 for the
// console list — which this query's leading `project_id` can use, though a
// dedicated (project_id, recipient_external_id, id DESC) would serve its cursor
// walk better. Measure before adding one: every index here is paid on the send
// hot path, which inserts into this table.
//
// (A commented-out `CREATE INDEX ... (id DESC, project_id, recipient_external_id)`
// sat here for a long time and was never applied — a comment is not a migration,
// and no runner is wired in. Its leading `id DESC` could not seek to a project
// anyway. Migrations live in migrations/, applied with goose.)
func (r *NotificationRepo) ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*entity.Notification, *query.Cursor, error) {
	returnedCursor := &query.Cursor{
		After:  nil,
		Before: nil,
	}

	b := dbx.NewSQLBuilder(`
		SELECT id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, read_at, opened_at, created_at, updated_at, completed_at, status
		FROM notification
	`)
	b.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	b.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)
	// Only surface notifications that were actually delivered (or are still in
	// flight) — see recipientFeedVisible.
	b.AppendWhere(recipientFeedVisible)

	if cursor.BeforeIsValid() && !cursor.AfterIsValid() {
		b.AddCompareFilter("id", dbx.OperatorLT, cursor.Before)
	}

	if cursor.AfterIsValid() && !cursor.BeforeIsValid() {
		b.AddCompareFilter("id", dbx.OperatorGT, cursor.After)
	}

	b.AddSorting("id", "DESC")
	b.AddPagination(*cursor.Limit+1, 0) // Overfetch by 1 to determine if there are more notifications.

	sql, args := b.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	notifications := []*entity.Notification{}
	for rows.Next() {
		var notification entity.Notification

		err := rows.Scan(&notification.ID, &notification.ProjectID, &notification.RecipientExtID,
			&notification.Payload, &notification.BroadcastID, &notification.Channel, &notification.Topic,
			&notification.Event, &notification.ReadAt, &notification.OpenedAt, &notification.CreatedAt,
			&notification.UpdatedAt, &notification.CompletedAt, &notification.Status)
		if err != nil {
			return nil, nil, fmt.Errorf("scan: %w", err)
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	hasMore := false
	if len(notifications) > *cursor.Limit {
		hasMore = true
		// Trim the overfetched notification.
		notifications = notifications[:*cursor.Limit]
	}

	if len(notifications) > 0 {
		firstNotification := notifications[0]
		lastNotification := notifications[len(notifications)-1]

		before := fmt.Sprintf("%d", lastNotification.ID)
		after := fmt.Sprintf("%d", firstNotification.ID)

		if hasMore {
			returnedCursor.Before = &before
		}

		// We are not at the start of  list if we have a 'before' cursor
		// that means there are newer items than the current first item.
		if cursor.BeforeIsValid() && !cursor.AfterIsValid() {
			returnedCursor.After = &after
		}
	}

	return notifications, returnedCursor, nil
}

func (r *NotificationRepo) UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error) {
	// Must stay in lockstep with ListForRecipient's predicate: a badge counting
	// rows the feed will not show is a bug the user experiences as an unread
	// count they cannot clear.
	sql := `
		SELECT COUNT(*) FROM notification
		WHERE project_id = $1 AND recipient_external_id = $2 AND read_at IS NULL
		  AND ` + recipientFeedVisible + `
	`
	var count int

	err := r.db.QueryRow(ctx, sql, projectID, recipientExtID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query and scan: %w", err)
	}

	return count, nil
}

// StatusRollupForBroadcast returns the per-status notification counts for one
// broadcast — the in_app branch of the console's delivery tree.
//
// Scoped by broadcast_id ALONE, with no project_id predicate, so it is served
// entirely from `ix_notification_broadcast (broadcast_id) INCLUDE (status)`
// without touching the heap. Ownership is the caller's job: the service loads
// the broadcast first and checks its project, which is where that check belongs
// anyway (a broadcast id that is not in the project must 404, not return zero
// counts and look like an empty broadcast).
func (r *NotificationRepo) StatusRollupForBroadcast(ctx context.Context, broadcastID int) (map[enum.NotificationStatus]int, error) {
	sql := `
		SELECT status, COUNT(*)
		FROM notification
		WHERE broadcast_id = $1
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, sql, broadcastID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	rollup := make(map[enum.NotificationStatus]int)
	for rows.Next() {
		var status enum.NotificationStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rollup[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return rollup, nil
}

// CountStuck counts notifications stalled in `enqueued` across all projects.
// See the interface doc in model/repository/notification.go for why it is not
// project-scoped.
//
// `enqueued` is the only non-terminal NotificationStatus, and the
// notification:delivery task normally resolves it sub-second, so a row still
// sitting there minutes later means the send path stopped after the initial
// INSERT — which is precisely what a dead or failing worker looks like from the
// database side.
//
// ⚠️ Deliberately NOT indexed. `notification` is on the send hot path and
// carries only two indexes for that reason (see agent-docs/overview.md), and a
// partial index on status='enqueued' would add an index write on INSERT plus an
// index delete on resolve to EVERY send — taxing the hot path to serve a query
// that runs once a minute. The `newerThan` bound keeps the scan proportional to
// recent volume rather than all history. If this ever shows up in query stats,
// the fix is `CREATE INDEX ... ON notification (created_at) WHERE status =
// 'enqueued'` — tiny, since it indexes only unresolved rows — but measure first.
func (r *NotificationRepo) CountStuck(ctx context.Context, olderThan, newerThan time.Time) (int, error) {
	sql := `
		SELECT COUNT(*) FROM notification
		WHERE status = 'enqueued'
		  AND created_at < $1
		  AND created_at > $2
	`

	var count int

	err := r.db.QueryRow(ctx, sql, olderThan, newerThan).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query and scan: %w", err)
	}

	return count, nil
}

func (r *NotificationRepo) UpdateForRecipient(ctx context.Context, projectID int, recipientExtID string, payload dto.UpdateRecipientNotificationsPayload) (int, error) {
	sql := `
		UPDATE notification
	`
	b := dbx.NewSQLBuilder(sql)
	b.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	b.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)
	// A recipient can only mark rows they can actually see. This query had NO
	// status predicate before, so mark-all-read stamped `read_at` on rows the
	// recipient was never shown — harmless while UnreadCountForRecipient excluded
	// them anyway, but wrong, and actively misleading once email-only rows exist
	// (marking an email delivery's bookkeeping row as "read by the recipient").
	//
	// Applied to the by-id form too, not just the bulk one: a recipient cannot
	// learn a hidden row's id through the API, so naming one is either a mistake
	// or an attempt to touch a row that is not theirs to mark.
	//
	// Behaviour change worth stating: the returned "updated" count is now the
	// number of VISIBLE rows changed, so a mark-all-read on a recipient with muted
	// rows reports a smaller number than it used to. That number is the honest one.
	b.AppendWhere(recipientFeedVisible)

	now := time.Now().UTC()

	if payload.State.Read != nil {
		if *payload.State.Read {
			b.SetColumn("read_at", now)
		} else {
			b.SetColumn("read_at", nil)
		}
	}

	if payload.State.Opened != nil {
		if *payload.State.Opened {
			b.SetColumn("opened_at", now)
		} else {
			b.SetColumn("opened_at", nil)
		}
	}

	if len(payload.IDs) == 0 {
		// If no notification IDs are provided, update all notifications for the recipient.
	} else {
		// Update only specific notifications
		ids := make([]any, len(payload.IDs))
		for i, id := range payload.IDs {
			ids[i] = id
		}
		b.AddArrayFilter("id", ids)
	}

	sql, args := b.Build()

	res, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("update notifications for recipient: %w", err)
	}

	return int(res.RowsAffected()), nil
}

// DeleteForRecipient deliberately does NOT apply recipientFeedVisible, unlike
// every other recipient-scoped method here.
//
// The clear-my-inbox path has always deleted `muted` and `quota_exceeded` rows
// along with visible ones, and those already cascade away any attached email
// delivery record (notification_delivery ON DELETE CASCADE). Excluding only the
// new `not_requested` status would make the behaviour inconsistent with itself
// without fixing anything.
//
// The real wart — that a recipient clearing their inbox destroys the operator's
// email delivery history — predates email-only sends and applies equally to a
// `muted` row whose send carried an email block. Fixing it means deciding
// whether delete should soft-delete or preserve deliveries, which is a change to
// shipped behaviour and belongs in its own unit.
func (r *NotificationRepo) DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error) {
	b := dbx.NewSQLBuilder("DELETE FROM notification")
	b.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	b.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)

	if len(notificationIDs) == 0 {
		// If no notification IDs are provided, delete all notifications for the recipient.
	} else {
		ids := make([]any, len(notificationIDs))
		for i, id := range notificationIDs {
			ids[i] = id
		}
		b.AddArrayFilter("id", ids)
	}

	sql, args := b.Build()
	res, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

func (r *NotificationRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM notification
		WHERE project_id = $1
	`
	res, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete notifications for project: %w", err)
	}
	return int(res.RowsAffected()), nil
}

// escapeLikeNeedle makes a user-typed search term match literally, by escaping
// the wildcards LIKE/ILIKE would otherwise honour inside it.
//
// This matters more than it looks: recipient external ids are customer-chosen
// and very commonly contain `_` (`user_1`, `perf_user_42`), which LIKE reads as
// "any single character" — so searching `user_1` would quietly also match
// `userX1`. `%` and the escape character itself have the same problem.
//
// Postgres' default LIKE escape character is a backslash, so prefixing is enough
// and no ESCAPE clause is needed. The needle is passed as a bind parameter, so
// this is about matching precision, not injection.
func escapeLikeNeedle(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func (r *NotificationRepo) ListNotifications(ctx context.Context, filters *dto.ListNotificationsFilters) ([]*entity.Notification, int, error) {
	sql := `
		SELECT id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, read_at, opened_at, created_at, updated_at, completed_at, status
		FROM notification
	`

	b := dbx.NewSQLBuilder(sql)

	b.AddCompareFilter("project_id", dbx.OperatorEQ, filters.ProjectID)

	switch filters.Kind {
	case enum.NotificationKindBroadcast:
		b.AppendWhere("broadcast_id IS NOT NULL")
	case enum.NotificationKindAll:
		// No predicate — both kinds.
	default:
		b.AppendWhere("broadcast_id IS NULL")
	}

	if filters.RecipientExtID != nil {
		b.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, *filters.RecipientExtID)
	}

	if filters.RecipientSearch != nil {
		b.AddContainsFilter("recipient_external_id", escapeLikeNeedle(*filters.RecipientSearch), false)
	}

	if filters.Status != nil {
		b.AddCompareFilter("status", dbx.OperatorEQ, string(*filters.Status))
	}

	if filters.Channel != nil {
		b.AddCompareFilter("channel", dbx.OperatorEQ, *filters.Channel)
	}
	if filters.Topic != nil {
		b.AddCompareFilter("topic", dbx.OperatorEQ, *filters.Topic)
	}
	if filters.Event != nil {
		b.AddCompareFilter("event", dbx.OperatorEQ, *filters.Event)
	}

	// Dereferenced deliberately: AddCompareFilter skips a nil `value`, but a nil
	// *time.Time boxed into `any` is NOT == nil, so passing the pointer would
	// sneak a NULL comparison into the WHERE and match nothing.
	if filters.CreatedFrom != nil {
		b.AddCompareFilter("created_at", dbx.OperatorGTE, *filters.CreatedFrom)
	}
	if filters.CreatedTo != nil {
		b.AddCompareFilter("created_at", dbx.OperatorLTE, *filters.CreatedTo)
	}

	// The email filter is an EXISTS subquery, NOT a join — and that is the whole
	// design of this method, so it is worth stating plainly.
	//
	// This list attaches email deliveries via a SECOND batch query (below),
	// because a notification with no email simply has no delivery row: joining
	// would drop every in-app-only notification, still the common case. Adding a
	// delivery-status filter does NOT force that join. EXISTS keeps the main
	// query on `notification` alone — same plan, same batch attach, one extra
	// semi-join — and it keeps the two concerns honest:
	//
	//   - narrowing the row set (this predicate) is what the operator ASKED for;
	//   - projecting the email data (the batch query) must never narrow anything.
	//
	// EmailFilterNone is the anti-join that makes in-app-only rows findable
	// rather than merely un-dropped.
	if filters.Email != nil {
		const emailDeliveryExists = `SELECT 1 FROM notification_delivery nd
			WHERE nd.notification_id = notification.id AND nd.medium = 'email'`

		switch *filters.Email {
		case enum.EmailFilterNone:
			b.AppendWhere(fmt.Sprintf("NOT EXISTS (%s)", emailDeliveryExists))
		case enum.EmailFilterAny:
			b.AppendWhere(fmt.Sprintf("EXISTS (%s)", emailDeliveryExists))
		default:
			// Validated as a real DeliveryStatus by the DTO.
			b.AppendWhere(
				fmt.Sprintf("EXISTS (%s AND nd.status = $%d)", emailDeliveryExists, b.ArgNum()),
				string(*filters.Email),
			)
		}
	}

	b.AddSorting("id", "DESC")
	b.AddPagination(filters.Pagination.Limit, filters.Pagination.Offset())

	sql, args := b.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	notifications := []*entity.Notification{}
	for rows.Next() {
		var notification entity.Notification

		err := rows.Scan(&notification.ID, &notification.ProjectID, &notification.RecipientExtID,
			&notification.Payload, &notification.BroadcastID, &notification.Channel, &notification.Topic,
			&notification.Event, &notification.ReadAt, &notification.OpenedAt, &notification.CreatedAt,
			&notification.UpdatedAt, &notification.CompletedAt, &notification.Status)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	// Attach the email-medium delivery outcome per notification in one batch
	// query (email is the only non-in_app medium written today). A notification
	// with no email send simply has no matching row, so EmailStatus stays nil.
	if len(notifications) > 0 {
		byID := make(map[int]*entity.Notification, len(notifications))
		ids := make([]int, len(notifications))
		for i, n := range notifications {
			ids[i] = n.ID
			byID[n.ID] = n
		}

		// Every BOUNDED delivery column is projected here, so the list can explain
		// an outcome inline (failure_reason) and the detail dialog opens without a
		// second fetch. provider_response is deliberately EXCLUDED — it is an
		// unbounded JSONB array of raw webhook bodies (Phase 5) and is served
		// per-notification instead (see agent-docs/overview.md, Phase 9.1).
		deliverySQL := `
			SELECT notification_id, status, failure_reason, attempt, provider, provider_message_id,
			       address_snapshot, sent_at, delivered_at, bounced_at, complained_at, opened_at, clicked_at
			FROM notification_delivery
			WHERE medium = 'email' AND notification_id = ANY($1)
		`
		drows, err := r.db.Query(ctx, deliverySQL, ids)
		if err != nil {
			return nil, 0, fmt.Errorf("query email deliveries: %w", err)
		}
		defer drows.Close()

		for drows.Next() {
			var (
				nid int
				d   entity.NotificationEmailDelivery
			)
			if err := drows.Scan(&nid, &d.Status, &d.FailureReason, &d.Attempt, &d.Provider,
				&d.ProviderMessageID, &d.AddressSnapshot, &d.SentAt, &d.DeliveredAt, &d.BouncedAt,
				&d.ComplainedAt, &d.OpenedAt, &d.ClickedAt); err != nil {
				return nil, 0, fmt.Errorf("scan email delivery: %w", err)
			}
			if n, ok := byID[nid]; ok {
				email := d
				n.Email = &email
			}
		}
		if err := drows.Err(); err != nil {
			return nil, 0, fmt.Errorf("email deliveries rows error: %w", err)
		}
	}

	countSQL, countArgs := b.Count()
	var total int
	err = r.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

func (r *NotificationRepo) Update(ctx context.Context, notification *entity.Notification) error {
	sql := `
		UPDATE notification
		SET payload = $3, channel = $4, topic = $5, event = $6, read_at = $7, opened_at = $8,
		updated_at = $9, completed_at = $10, status = $11
		WHERE id = $1 AND project_id = $2
	`
	_, err := r.db.Exec(ctx, sql,
		notification.ID, notification.ProjectID, notification.Payload, notification.Channel,
		notification.Topic, notification.Event, notification.ReadAt, notification.OpenedAt,
		notification.UpdatedAt, notification.CompletedAt, notification.Status,
	)
	return err
}
