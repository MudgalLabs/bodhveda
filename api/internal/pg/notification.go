package pg

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type NotificationRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewNotificationRepo(db *pgxpool.Pool) repository.NotificationRepository {
	return &APIKeyRepo{
		db:   db,
		pool: db,
	}
}
