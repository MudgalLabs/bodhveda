package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type UserSubscriptionRepository interface {
	UserSubscriptionReader
	UserSubscriptionWriter
}

type UserSubscriptionReader interface {
	Get(ctx context.Context, userID int) (*entity.UserSubscription, error)
	// ⚠️ Read inside the caller's transaction by CheckAndConsumeUsageTx — see there.
	GetTx(ctx context.Context, tx pgx.Tx, userID int) (*entity.UserSubscription, error)
}

type UserSubscriptionWriter interface {
	Upsert(ctx context.Context, sub *entity.UserSubscription) error
	UpsertTx(ctx context.Context, tx pgx.Tx, sub *entity.UserSubscription) error
}

type SearchUserSubscriptionFilter struct {
	UserID *int `json:"user_id"`
}

type SearchUserSubscriptionPayload = query.SearchPayload[SearchUserSubscriptionFilter]
