package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/logger"
)

const (
	TaskTypeBroadcastDelivery = "broadcast:delivery"
)

type BroadcastDeliveryProcessor struct {
	db                 *pgxpool.Pool
	notificationRepo   repository.NotificationRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
}

func NewBroadcastDeliveryProcessor(
	db *pgxpool.Pool, notificationRepo repository.NotificationRepository,
	broadcastRepo repository.BroadcastRepository, broadcastBatchRepo repository.BroadcastBatchRepository,
) *BroadcastDeliveryProcessor {
	return &BroadcastDeliveryProcessor{
		db:                 db,
		notificationRepo:   notificationRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
	}
}

func (processor *BroadcastDeliveryProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	start := time.Now()

	var payload entity.BroadcastDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	err := dbx.WithTx(ctx, processor.db, func(tx pgx.Tx) error {
		notifications := make([]*entity.Notification, 0, len(payload.RecipientExtIDs))

		for _, recipientExtID := range payload.RecipientExtIDs {
			notifications = append(notifications, entity.NewNotification(
				payload.ProjectID, recipientExtID, payload.Payload,
				&payload.BroadcastID, payload.Channel,
				payload.Topic, payload.Event,
			))
		}

		err := processor.notificationRepo.BatchCreateTx(ctx, tx, notifications)
		if err != nil {
			err = fmt.Errorf("batch create notifications: %w", err)
			logger.Get().Error(err)
			return err
		}

		return err
	})

	duration := time.Since(start)

	var status enum.BroadcastBatchStatus
	if err != nil {
		status = enum.BroadcastBatchStatusFailed
	} else {
		status = enum.BroadcastBatchStatusCompleted
	}

	attempt := 1
	count, ok := asynq.GetRetryCount(ctx)
	if ok {
		attempt = count + 1
	}

	err = processor.broadcastBatchRepo.Update(ctx, payload.BatchID, entity.NewBroadcastBatchUpdatePayload(
		status, attempt, int(duration.Milliseconds()),
	))
	if err != nil {
		err = fmt.Errorf("update broadcast batch status: %w", err)
		logger.Get().Error(err)
		return err
	}

	remaining, err := processor.broadcastBatchRepo.PendingCount(ctx, payload.BroadcastID)
	if err != nil {
		err = fmt.Errorf("count pending batches: %w", err)
		logger.Get().Error(err)
	}

	// All batches processed, we can mark the broadcast as completed.
	if remaining == 0 {
		broadcast, err := processor.broadcastRepo.GetByID(ctx, payload.BroadcastID)
		if err != nil {
			err = fmt.Errorf("get broadcast by ID: %w", err)
			logger.Get().Error(err)
		}

		now := time.Now().UTC()
		broadcast.CompletedAt = &now
		broadcast.UpdatedAt = now

		err = processor.broadcastRepo.Update(ctx, broadcast)
		if err != nil {
			err = fmt.Errorf("update broadcast: %w", err)
			logger.Get().Error(err)
			return err
		}
	}

	return nil
}
