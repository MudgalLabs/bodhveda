package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/query"
)

type BroadcastRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewBroadcastRepo(db *pgxpool.Pool) repository.BroadcastRepository {
	return &BroadcastRepo{
		db:   db,
		pool: db,
	}
}

func (r *BroadcastRepo) Create(ctx context.Context, broadcast *entity.Broadcast) (*entity.Broadcast, error) {
	sql := `
		INSERT INTO broadcast (
			project_id, payload, channel, topic, event, completed_at, created_at, updated_at, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, project_id, payload, channel, topic, event, completed_at, created_at, updated_at, status
	`
	row := r.db.QueryRow(ctx, sql, broadcast.ProjectID, broadcast.Payload, broadcast.Channel, broadcast.Topic,
		broadcast.Event, broadcast.CompletedAt, broadcast.CreatedAt, broadcast.UpdatedAt, broadcast.Status,
	)

	var newBroadcast entity.Broadcast

	err := row.Scan(&newBroadcast.ID, &newBroadcast.ProjectID, &newBroadcast.Payload, &newBroadcast.Channel,
		&newBroadcast.Topic, &newBroadcast.Event, &newBroadcast.CompletedAt, &newBroadcast.CreatedAt,
		&newBroadcast.UpdatedAt, &newBroadcast.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("scan broadcast: %w", err)
	}

	return &newBroadcast, nil
}

func (r *BroadcastRepo) GetByID(ctx context.Context, id int) (*entity.Broadcast, error) {
	sql := `
		SELECT id, project_id, payload, channel, topic, event, completed_at, created_at,
		updated_at, status
		FROM broadcast
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, sql, id)

	var broadcast entity.Broadcast

	err := row.Scan(&broadcast.ID, &broadcast.ProjectID, &broadcast.Payload, &broadcast.Channel, &broadcast.Topic,
		&broadcast.Event, &broadcast.CompletedAt, &broadcast.CreatedAt, &broadcast.UpdatedAt, &broadcast.Status)
	if err != nil {
		return nil, fmt.Errorf("scan broadcast by id: %w", err)
	}

	return &broadcast, nil
}

func (r *BroadcastRepo) Update(ctx context.Context, broadcast *entity.Broadcast) error {
	sql := `
		UPDATE broadcast
		SET payload = $2, channel = $3, topic = $4, event = $5, completed_at = $6,
		updated_at = $7, status = $8
		WHERE id = $1
	`
	_, err := r.db.Exec(
		ctx, sql, broadcast.ID, broadcast.Payload, broadcast.Channel, broadcast.Topic, broadcast.Event,
		broadcast.CompletedAt, broadcast.UpdatedAt, broadcast.Status,
	)
	return err
}

func (r *BroadcastRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM broadcast
		WHERE project_id = $1
	`
	tag, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete broadcasts for project: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func (r *BroadcastRepo) List(ctx context.Context, projectID int, pagination query.Pagination) ([]*dto.BroadcastListItem, int, error) {
	sql := `
		SELECT 
			id, payload, channel, topic, event, completed_at, created_at, updated_at, status
		FROM broadcast
	`
	b := dbx.NewSQLBuilder(sql)
	b.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	b.AddSorting("id", "DESC")
	b.AddPagination(pagination.Limit, pagination.Offset())

	sql, args := b.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	broadcasts := []*dto.BroadcastListItem{}
	broadcastIDs := []int{}
	for rows.Next() {
		var broadcast dto.BroadcastListItem
		err := rows.Scan(
			&broadcast.ID, &broadcast.Payload, &broadcast.Target.Channel, &broadcast.Target.Topic,
			&broadcast.Target.Event, &broadcast.CompletedAt, &broadcast.CreatedAt, &broadcast.UpdatedAt,
			&broadcast.Status,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		broadcasts = append(broadcasts, &broadcast)
		broadcastIDs = append(broadcastIDs, broadcast.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	countsSQL := `
		SELECT 
			broadcast_id,
			COUNT(id) AS delivered_count,
			COUNT(id) FILTER (WHERE read_at IS NOT NULL) AS read_count,
			COUNT(id) FILTER (WHERE opened_at IS NOT NULL) AS opened_count
		FROM notification
		WHERE broadcast_id = ANY($1)
		GROUP BY broadcast_id
	`

	countRows, err := r.db.Query(ctx, countsSQL, broadcastIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("notification counts: %w", err)
	}

	defer countRows.Close()

	type counts struct{ delivered, read, opened int }
	countMap := make(map[int]counts)

	for countRows.Next() {
		var bid, delivered, read, opened int

		if err := countRows.Scan(&bid, &delivered, &read, &opened); err != nil {
			return nil, 0, fmt.Errorf("scan counts: %w", err)
		}

		countMap[bid] = counts{delivered, read, opened}
	}

	if err := countRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("counts rows error: %w", err)
	}

	// Attach counts to broadcasts
	for _, b := range broadcasts {
		if c, ok := countMap[b.ID]; ok {
			b.DeliveredCount = c.delivered
			b.ReadCount = c.read
			b.OpenedCount = c.opened
		}
	}

	countSQL, countArgs := b.Count()
	var total int

	err = r.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return broadcasts, total, nil
}
