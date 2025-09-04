package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type UserSubscriptionRepository interface {
	UserSubscriptionReader
	UserSubscriptionWriter
}

type UserSubscriptionReader interface {
	Get(ctx context.Context, userID int) (*entity.UserSubscription, error)
}

type UserSubscriptionWriter interface {
	Upsert(ctx context.Context, sub *entity.UserSubscription) error
}

type SearchUserSubscriptionFilter struct {
	UserID *int `json:"user_id"`
}

type SearchUserSubscriptionPayload = query.SearchPayload[SearchUserSubscriptionFilter]
