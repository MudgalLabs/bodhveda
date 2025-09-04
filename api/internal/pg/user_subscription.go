package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type UserSubscriptionRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewUserSubscriptionRepo(db *pgxpool.Pool) repository.UserSubscriptionRepository {
	return &UserSubscriptionRepo{
		db:   db,
		pool: db,
	}
}

func (r *UserSubscriptionRepo) Get(ctx context.Context, userID int) (*entity.UserSubscription, error) {
	payload := repository.SearchUserSubscriptionPayload{
		Filters: repository.SearchUserSubscriptionFilter{
			UserID: &userID,
		},
	}

	subs, _, err := r.findSubscriptions(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("find subscriptions: %w", err)
	}

	if len(subs) == 0 {
		return nil, tantraRepo.ErrNotFound
	}

	return subs[0], nil
}

func (r *UserSubscriptionRepo) Upsert(ctx context.Context, sub *entity.UserSubscription) error {
	sql := `
		INSERT INTO user_subscription
		(user_id, plan_id, current_period_start, current_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET plan_id = EXCLUDED.plan_id, current_period_start = EXCLUDED.current_period_start,
		current_period_end = EXCLUDED.current_period_end, updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.Exec(ctx, sql, sub.UserID, sub.PlanID, sub.CurrentPeriodStart,
		sub.CurrentPeriodEnd, sub.CreatedAt, sub.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	return nil
}

func (r *UserSubscriptionRepo) findSubscriptions(ctx context.Context, payload repository.SearchUserSubscriptionPayload) ([]*entity.UserSubscription, int, error) {
	sql := `
		SELECT user_id, plan_id, current_period_start, current_period_end, updated_at, created_at
		FROM user_subscription
	`

	builder := dbx.NewSQLBuilder(sql)

	if payload.Filters.UserID != nil && *payload.Filters.UserID > 0 {
		builder.AddCompareFilter("user_id", dbx.OperatorEQ, *payload.Filters.UserID)
	}

	// Apply default pagination if not provided.
	if payload.Pagination.Limit <= 0 {
		payload.Pagination.Limit = 20
	}

	if payload.Pagination.Page <= 0 {
		payload.Pagination.Page = 1
	}

	builder.AddPagination(payload.Pagination.Limit, payload.Pagination.Offset())
	builder.AddSorting(payload.Sort.Field, payload.Sort.Order)

	sql, args := builder.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	subs := []*entity.UserSubscription{}
	for rows.Next() {
		var sub entity.UserSubscription
		err := rows.Scan(&sub.UserID, &sub.PlanID, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.UpdatedAt, &sub.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		subs = append(subs, &sub)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	countSQL, countArgs := builder.Count()
	var total int
	err = r.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return subs, total, nil
}
