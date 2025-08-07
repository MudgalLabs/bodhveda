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
)

const (
	TaskTypeBroadcastDelivery = "broadcast:delivery"
)

type BroadcastProcessor struct {
	db                 *pgxpool.Pool
	notificationRepo   repository.NotificationRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
}

func NewBroadcastProcessor(db *pgxpool.Pool, notificationRepo repository.NotificationRepository, broadcastBatchRepo repository.BroadcastBatchRepository) *BroadcastProcessor {
	return &BroadcastProcessor{
		db:                 db,
		notificationRepo:   notificationRepo,
		broadcastBatchRepo: broadcastBatchRepo,
	}
}

func (processor *BroadcastProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	start := time.Now()

	var payload entity.BroadcastDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
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
			return fmt.Errorf("batch create notifications: %w", err)
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

	err = processor.broadcastBatchRepo.Update(ctx, payload.BatchID, status, int(duration.Milliseconds()))
	if err != nil {
		return fmt.Errorf("update broadcast batch status: %w", err)
	}

	return nil
}
