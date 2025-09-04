// Package processor contains Asynq task processors for handling various background jobs.
package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/logger"
)

type NotificationDeliveryProcessor struct {
	db               *pgxpool.Pool
	notificationRepo repository.NotificationRepository
	preferenceRepo   repository.PreferenceRepository
	billingService   *service.BillingService
}

func NewNotificationDeliveryProcessor(
	db *pgxpool.Pool,
	notificationRepo repository.NotificationRepository, preferenceRepo repository.PreferenceRepository,
	billingService *service.BillingService,
) *NotificationDeliveryProcessor {
	return &NotificationDeliveryProcessor{
		db:               db,
		notificationRepo: notificationRepo,
		preferenceRepo:   preferenceRepo,
		billingService:   billingService,
	}
}

func (processor *NotificationDeliveryProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.NotificationDeliveryTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	notification := payload.Notification

	shouldDeliver, err := processor.preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, notification.ProjectID, notification.RecipientExtID, dto.TargetFromNotification(notification))
	if err != nil {
		return err
	}

	if !shouldDeliver {
		notification.Status = enum.NotificationStatusMuted
	} else {
		event := dto.UsageEvent{
			UserID:    payload.UserID,
			ProjectID: notification.ProjectID,
			Metric:    entity.MetricNotifications,
			Amount:    1,
		}

		err := processor.billingService.CheckAndConsumeUsage(ctx, event)
		if err != nil {
			if errors.Is(err, enum.ErrQuotaExceeded) {
				notification.Status = enum.NotificationStatusQuotaExceeded
			} else {
				err = fmt.Errorf("check and consume usage: %w", err)
				logger.Get().Error(err)
				return err
			}
		} else {
			notification.Status = enum.NotificationStatusDelivered
		}
	}

	now := time.Now().UTC()
	notification.CompletedAt = &now

	err = processor.notificationRepo.Update(ctx, notification)
	if err != nil {
		return fmt.Errorf("update notification: %w", err)
	}

	logger.Get().Infof("NotificationDeliveryProcessor: Successfully completed notification %d", notification.ID)

	return nil
}

type PrepareBroadcastBatchesProcessor struct {
	db                 *pgxpool.Pool
	asynqClient        *asynq.Client
	preferenceRepo     repository.PreferenceRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
	billingService     *service.BillingService
}

func NewPrepareBroadcastBatchesProcessor(
	db *pgxpool.Pool, asynqClient *asynq.Client, preferenceRepo repository.PreferenceRepository,
	broadcastRepo repository.BroadcastRepository, broadcastBatchRepo repository.BroadcastBatchRepository,
	billingService *service.BillingService,
) *PrepareBroadcastBatchesProcessor {
	return &PrepareBroadcastBatchesProcessor{
		db:                 db,
		asynqClient:        asynqClient,
		preferenceRepo:     preferenceRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
		billingService:     billingService,
	}
}

func (processor *PrepareBroadcastBatchesProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.PrepareBroadcastBatchesPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	broadcast := payload.Broadcast

	recipientExtIDs, err := processor.preferenceRepo.ListEligibleRecipientExtIDsForBroadcast(ctx, broadcast.ProjectID, dto.TargetFromBroadcast(broadcast))
	if err != nil {
		err = fmt.Errorf("list eligible recipient external IDs: %w", err)
		logger.Get().Error(err)
		return err
	}

	event := dto.UsageEvent{
		UserID:    payload.UserID,
		ProjectID: broadcast.ProjectID,
		Metric:    entity.MetricNotifications,
		Amount:    int64(len(recipientExtIDs)),
	}

	err = processor.billingService.CheckAndConsumeUsage(ctx, event)
	if err != nil {
		if errors.Is(err, enum.ErrQuotaExceeded) {
			broadcast.Status = enum.BroadcastStatusQuotaExceeded
		} else {
			err = fmt.Errorf("check and consume usage: %w", err)
			logger.Get().Error(err)
			return err
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

		logger.Get().Infof("PrepareBroadcastBatchesProcessor: Successfully processed task for broadcast %d with %d recipients", broadcast.ID, len(recipientExtIDs))
		return nil
	}

	var batchSize int

	if len(recipientExtIDs) <= 100 {
		batchSize = len(recipientExtIDs)
	} else {
		batchSize = min(max(len(recipientExtIDs)/10, 100), 1000)
	}

	for i := 0; i < len(recipientExtIDs); i += batchSize {
		end := min(i+batchSize, len(recipientExtIDs))

		recipientsBatch := recipientExtIDs[i:end]
		broadcastBatch := entity.NewBroadcastBatch(broadcast.ID, len(recipientsBatch))

		broadcastBatch, err := processor.broadcastBatchRepo.Create(ctx, broadcastBatch)
		if err != nil {
			err = fmt.Errorf("create broadcast batch: %w", err)
			logger.Get().Error(err)
			return err
		}

		payload, err := json.Marshal(dto.BroadcastDeliveryTaskPayload{
			ProjectID:       broadcast.ProjectID,
			BroadcastID:     broadcast.ID,
			BatchID:         broadcastBatch.ID,
			RecipientExtIDs: recipientsBatch,
			Payload:         broadcast.Payload,
			Channel:         broadcast.Channel,
			Topic:           broadcast.Topic,
			Event:           broadcast.Event,
		})
		if err != nil {
			err = fmt.Errorf("marshal broadcast delivery task payload: %w", err)
			logger.Get().Error(err)
			return err
		}

		task := asynq.NewTask(task.TaskTypeBroadcastDelivery, payload)

		_, err = processor.asynqClient.Enqueue(task, asynq.MaxRetry(3))
		if err != nil {
			err = fmt.Errorf("enqueue broadcast delivery task: %w", err)
			logger.Get().Error(err)
			return err
		}
	}

	logger.Get().Infof("PrepareBroadcastBatchesProcessor: Successfully processed task for broadcast %d with %d recipients", broadcast.ID, len(recipientExtIDs))
	return nil
}

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

	var payload dto.BroadcastDeliveryTaskPayload
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
				&payload.BroadcastID, payload.Channel, payload.Topic, payload.Event,
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
		status = enum.BroadcastBatchStatusSuccess
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
		broadcast.Status = enum.BroadcastStatusCompleted
		broadcast.UpdatedAt = now

		err = processor.broadcastRepo.Update(ctx, broadcast)
		if err != nil {
			err = fmt.Errorf("update broadcast: %w", err)
			logger.Get().Error(err)
			return err
		}

		logger.Get().Infof("BroadcastDeliveryProcessor: Successfully completed broadcast %d", payload.BroadcastID)
	}

	return nil
}

type DeleteRecipientDataProcessor struct {
	db               *pgxpool.Pool
	notificationRepo repository.NotificationRepository
	preferenceRepo   repository.PreferenceRepository
	recipientRepo    repository.RecipientRepository
}

func NewDeleteRecipientDataProcessor(
	preferenceRepo repository.PreferenceRepository, notificationRepo repository.NotificationRepository,
	recipientRepo repository.RecipientRepository,
) *DeleteRecipientDataProcessor {
	return &DeleteRecipientDataProcessor{
		notificationRepo: notificationRepo,
		preferenceRepo:   preferenceRepo,
		recipientRepo:    recipientRepo,
	}
}

func (processor *DeleteRecipientDataProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	l := logger.Get()

	var payload dto.DeleteRecipientDataPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	// 1. Delete preferences for the recipient.
	count, err := processor.preferenceRepo.DeleteForRecipient(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		err = fmt.Errorf("delete preferences for recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d preferences for recipient %s in project %d", count, payload.RecipientExtID, payload.ProjectID)

	// 2. Delete notifications for the recipient.
	count, err = processor.notificationRepo.DeleteForRecipient(ctx, payload.ProjectID, payload.RecipientExtID, nil)
	if err != nil {
		err = fmt.Errorf("delete notifications for recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d notifications for recipient %s in project %d", count, payload.RecipientExtID, payload.ProjectID)

	// 3. Finally, delete the recipient itself.
	err = processor.recipientRepo.Delete(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		err = fmt.Errorf("delete recipient: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted recipient %s in project %d", payload.RecipientExtID, payload.ProjectID)

	l.Infof("DeleteRecipientDataProcessor: Successfully deleted all data for recipient %s in project %d", payload.RecipientExtID, payload.ProjectID)
	return nil
}

type DeleteProjectDataProcessor struct {
	apikeyRepo         repository.APIKeyRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository
	notificationRepo   repository.NotificationRepository
	preferenceRepo     repository.PreferenceRepository
	projectRepo        repository.ProjectRepository
	recipientRepo      repository.RecipientRepository
}

func NewDeleteProjectDataProcessor(
	apikeyRepo repository.APIKeyRepository,
	broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository,
	notificationRepo repository.NotificationRepository,
	preferenceRepo repository.PreferenceRepository,
	projectRepo repository.ProjectRepository,
	recipientRepo repository.RecipientRepository,
) *DeleteProjectDataProcessor {
	return &DeleteProjectDataProcessor{
		apikeyRepo:         apikeyRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
		notificationRepo:   notificationRepo,
		preferenceRepo:     preferenceRepo,
		projectRepo:        projectRepo,
		recipientRepo:      recipientRepo,
	}
}

func (processor *DeleteProjectDataProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	l := logger.Get()

	var payload dto.DeleteProjectDataPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		err = fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		logger.Get().Error(err)
		return err
	}

	// 1. Delete API keys for the project.
	count, err := processor.apikeyRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete API keys for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d API keys for project %d", count, payload.ProjectID)

	// 2. Delete notifications for the project.
	count, err = processor.notificationRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete notifications for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d notifications for project %d", count, payload.ProjectID)

	// 3. Delete preferences for the project.
	count, err = processor.preferenceRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete preferences for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d preferences for project %d", count, payload.ProjectID)

	// 4. Delete recipients for the project.
	count, err = processor.recipientRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete recipients for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d recipients for project %d", count, payload.ProjectID)

	// 5. Delete broadcast batches for the project.
	count, err = processor.broadcastBatchRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete broadcast batches for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d broadcast batches for project %d", count, payload.ProjectID)

	// 6. Delete broadcasts for the project.
	count, err = processor.broadcastRepo.DeleteForProject(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete broadcasts for project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted %d broadcasts for project %d", count, payload.ProjectID)

	// 7. Finally, delete the project itself.
	err = processor.projectRepo.Delete(ctx, payload.ProjectID)
	if err != nil {
		err = fmt.Errorf("delete project: %w", err)
		l.Error(err)
		return err
	}
	l.Infof("Deleted project %d", payload.ProjectID)

	l.Infof("DeleteProjectDataProcessor: Successfully deleted all data for project %d", payload.ProjectID)
	return nil
}
