package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/query"
	"github.com/mudgallabs/tantra/service"
)

type NotificationService struct {
	repo               repository.NotificationRepository
	recipientRepo      repository.RecipientRepository
	preferenceRepo     repository.PreferenceRepository
	broadcastRepo      repository.BroadcastRepository
	broadcastBatchRepo repository.BroadcastBatchRepository

	billingService   *BillingService
	recipientService *RecipientService

	asynqClient *asynq.Client
}

func NewNotificationService(
	repo repository.NotificationRepository, recipientRepo repository.RecipientRepository,
	preferenceRepo repository.PreferenceRepository, broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository,
	billingService *BillingService, recipientService *RecipientService,
	asynqClient *asynq.Client,
) *NotificationService {
	return &NotificationService{
		repo:               repo,
		recipientRepo:      recipientRepo,
		preferenceRepo:     preferenceRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,

		billingService:   billingService,
		recipientService: recipientService,

		asynqClient: asynqClient,
	}
}

func (s *NotificationService) Send(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.SendNotificationResult, string, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, "", service.ErrInvalidInput, err
	}

	result := &dto.SendNotificationResult{}

	if payload.IsDirect() {
		result.Notification, err = s.sendDirectNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send direct notification: %w", err)
		}
	} else {
		// Check if a project preference exists that matches the target.
		// If not, we should return an error as no recipients would be able to receive this broadcast.
		prefExists, err := s.preferenceRepo.DoesProjectPreferenceExist(ctx, payload.ProjectID, *payload.Target)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("check if project preference exists: %w", err)
		}

		if !prefExists {
			return nil, "", service.ErrBadRequest, errors.New("No project preference exists that matches the target. Create a project preference first.")
		}

		result.Broadcast, err = s.sendBroadcastNotification(ctx, userID, payload)
		if err != nil {
			return nil, "", service.ErrInternalServerError, fmt.Errorf("send broadcast notification: %w", err)
		}
	}

	var message string
	if payload.IsDirect() {
		if result.Notification != nil {
			message = fmt.Sprintf("Direct notification sent successfully to recipient %s.", result.Notification.RecipientExtID)
		} else {
			message = "No notification was sent. Recipient's preferences do not allow delivery."
		}
	} else if payload.IsBroadcast() {
		if result.Broadcast != nil {
			message = "Broadcast notification sent successfully. It will be delivered to all elligible recipients."
		}
	}

	return result, message, service.ErrNone, nil
}

func (s *NotificationService) sendDirectNotification(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.Notification, error) {
	// This is to ensure that we can send notifications to recipients that are not yet created.
	_, _, err := s.recipientService.CreateIfNotExists(ctx, dto.CreateRecipientPayload{
		ProjectID:  payload.ProjectID,
		ExternalID: *payload.RecipientExtID,
	})

	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}

	var channel, topic, event string
	if payload.Target != nil {
		channel = payload.Target.Channel
		topic = payload.Target.Topic
		event = payload.Target.Event
	}

	notification := entity.NewNotification(
		payload.ProjectID,
		*payload.RecipientExtID,
		payload.Payload,
		nil,
		channel,
		topic,
		event,
	)

	notification, err = s.repo.Create(ctx, notification)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	taskPayload, err := json.Marshal(dto.NotificationDeliveryTaskPayload{UserID: userID, Notification: notification})
	if err != nil {
		return nil, fmt.Errorf("marshal notification delivery task payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypeNotificationDelivery, taskPayload)

	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(5))
	if err != nil {
		return nil, fmt.Errorf("enqueue notification delivery task: %w", err)
	}

	return dto.FromNotification(notification), nil
}

func (s *NotificationService) sendBroadcastNotification(ctx context.Context, userID int, payload dto.SendNotificationPayload) (*dto.Broadcast, error) {
	broadcast := entity.NewBroadcast(
		payload.ProjectID,
		payload.Payload,
		payload.Target.Channel,
		payload.Target.Topic,
		payload.Target.Event,
	)

	broadcast, err := s.broadcastRepo.Create(ctx, broadcast)
	if err != nil {
		return nil, fmt.Errorf("create broadcast: %w", err)
	}

	taskPayload, err := json.Marshal(dto.PrepareBroadcastBatchesPayload{UserID: userID, Broadcast: broadcast})
	if err != nil {
		return nil, fmt.Errorf("marshal prepare broadcast batches task payload: %w", err)
	}

	task := asynq.NewTask(task.TaskTypePrepareBroadcastBatches, taskPayload)

	_, err = s.asynqClient.Enqueue(task, asynq.MaxRetry(5))
	if err != nil {
		return nil, fmt.Errorf("enqueue prepare broadcast batches task: %w", err)
	}

	return dto.FromBroadcast(broadcast), nil
}

func (s *NotificationService) Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, service.Error, error) {
	result, err := s.repo.Overview(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("notification repo overview: %w", err)
	}
	return result, service.ErrNone, nil
}

func (s *NotificationService) ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*dto.Notification, *query.Cursor, service.Error, error) {
	if recipientExtID == "" {
		return nil, nil, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	err := cursor.Validate(100, 10)
	if err != nil {
		return nil, nil, service.ErrInvalidInput, err
	}

	notifs, returnedCursor, err := s.repo.ListForRecipient(ctx, projectID, recipientExtID, cursor)
	if err != nil {
		return nil, nil, service.ErrInternalServerError, err
	}

	return dto.FromNotifications(notifs), returnedCursor, service.ErrNone, nil
}

func (s *NotificationService) UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, service.Error, error) {
	if recipientExtID == "" {
		return 0, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	count, err := s.repo.UnreadCountForRecipient(ctx, projectID, recipientExtID)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return count, service.ErrNone, nil
}

func (s *NotificationService) UpdateForRecipient(ctx context.Context, projectID int, recipientExtID string, payload dto.UpdateRecipientNotificationsPayload) (int, service.Error, error) {
	updated, err := s.repo.UpdateForRecipient(ctx, projectID, recipientExtID, payload)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, service.Error, error) {
	updated, err := s.repo.DeleteForRecipient(ctx, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) ListNotifications(ctx context.Context, payload *dto.ListNotificationsFilters) (*dto.ListNotificationsResult, service.Error, error) {
	payload.Pagination.ApplyDefaults()

	notifications, total, err := s.repo.ListNotifications(ctx, payload.ProjectID, payload.Kind, payload.Pagination)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	return &dto.ListNotificationsResult{
		Notifications: dto.FromNotifications(notifications),
		Pagination:    payload.Pagination.GetMeta(total),
	}, service.ErrNone, nil
}
