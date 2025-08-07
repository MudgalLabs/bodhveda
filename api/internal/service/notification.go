package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/jobs"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type NotificationService struct {
	repo               repository.NotificationRepository
	recipientRepo      repository.RecipientRepository
	preferenceRepo     repository.PreferenceRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository

	asynqClient *asynq.Client
}

func NewNotificationService(
	repo repository.NotificationRepository, recipientRepo repository.RecipientRepository,
	preferenceRepo repository.PreferenceRepository, broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository,
	asynqClient *asynq.Client,
) *NotificationService {
	return &NotificationService{
		repo:               repo,
		recipientRepo:      recipientRepo,
		preferenceRepo:     preferenceRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,

		asynqClient: asynqClient,
	}
}

func (s *NotificationService) Send(ctx context.Context, payload dto.SendNotificationPayload) (*dto.SendNotificationResult, string, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, "", service.ErrInvalidInput, err
	}

	result := &dto.SendNotificationResult{}

	if payload.IsDirect() {
		result.Notification, err = s.sendDirectNotification(ctx, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send direct notification: %w", err)
		}
	} else {
		result.Broadcast, err = s.sendBroadcastNotification(ctx, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send broadcast notification: %w", err)
		}
	}

	var message string
	if payload.IsDirect() {
		if result.Notification != nil {
			message = fmt.Sprintf("Direct notification sent successfully to recipient %s", result.Notification.RecipientExtID)
		} else {
			message = "No notification sent, as recipient preferences do not allow delivery"
		}
	} else if payload.IsBroadcast() {
		if result.Broadcast != nil {
			message = "Broadcast notification sent successfully"
		}
	}

	return result, message, service.ErrNone, nil
}

func (s *NotificationService) sendDirectNotification(ctx context.Context, payload dto.SendNotificationPayload) (*dto.Notification, error) {
	notification := entity.NewNotification(
		payload.ProjectID,
		*payload.To.RecipientExtID,
		payload.Payload,
		nil,
		payload.To.Channel,
		payload.To.Topic,
		payload.To.Event,
	)

	shouldDeliver, err := s.preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, notification)
	if err != nil {
		return nil, err
	}

	if !shouldDeliver {
		// Can return nil here, as the notification will not be delivered.
		// The result will have nil `notification` field in `SendNotificationResult`.
		return nil, nil
	}

	// Creating a notification = sending it.
	notification, err = s.repo.Create(ctx, notification)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	return dto.FromNotification(notification), nil
}

func (s *NotificationService) sendBroadcastNotification(ctx context.Context, payload dto.SendNotificationPayload) (*dto.Broadcast, error) {
	broadcast := entity.NewBroadcast(
		payload.ProjectID,
		payload.Payload,
		payload.To.Channel,
		payload.To.Topic,
		payload.To.Event,
	)

	broadcast, err := s.broadcastRepo.Create(ctx, broadcast)
	if err != nil {
		return nil, fmt.Errorf("create broadcast: %w", err)
	}

	recipientExtIDs, err := s.preferenceRepo.ListEligibleRecipientExtIDsForBroadcast(ctx, broadcast)
	if err != nil {
		return nil, fmt.Errorf("list eligible recipient external IDs: %w", err)
	}

	const batchSize = 100

	for i := 0; i < len(recipientExtIDs); i += batchSize {
		end := min(i+batchSize, len(recipientExtIDs))

		recipientsBatch := recipientExtIDs[i:end]
		broadcastBatch := entity.NewBroadcastBatch(broadcast.ID)

		broadcastBatch, err := s.broadcastBatchRepo.Create(ctx, broadcastBatch)
		if err != nil {
			return nil, fmt.Errorf("create broadcast batch: %w", err)
		}

		payload, err := json.Marshal(entity.NewBroadcastDeliveryTaskPayload(
			payload.ProjectID,
			broadcast.ID,
			broadcastBatch.ID,
			recipientsBatch,
			payload.Payload,
			payload.To.Channel,
			payload.To.Topic,
			payload.To.Event,
		))
		if err != nil {
			return nil, fmt.Errorf("marshal notification batch payload: %w", err)
		}

		task := asynq.NewTask(jobs.TaskTypeBroadcastDelivery, payload)

		_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(3))
		if err != nil {
			return nil, fmt.Errorf("enqueue broadcast delivery task: %w", err)
		}
	}

	return dto.FromBroadcast(broadcast), nil
}
