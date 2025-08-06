package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type NotificationRepository interface {
	NotificationReader
	NotificationWriter
}

type NotificationReader interface {
}

type NotificationWriter interface {
	Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error)
}
