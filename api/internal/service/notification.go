package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	jobs "github.com/mudgallabs/bodhveda/internal/job"
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

	recipientService *RecipientService

	asynqClient *asynq.Client
}

func NewNotificationService(
	repo repository.NotificationRepository, recipientRepo repository.RecipientRepository,
	preferenceRepo repository.PreferenceRepository, broadcastRepo repository.BroadcastRepository,
	broadcastBatchRepo repository.BroadcastBatchRepository, recipientService *RecipientService,
	asynqClient *asynq.Client,
) *NotificationService {
	return &NotificationService{
		repo:               repo,
		recipientRepo:      recipientRepo,
		preferenceRepo:     preferenceRepo,
		broadcastRepo:      broadcastRepo,
		broadcastBatchRepo: broadcastBatchRepo,
		recipientService:   recipientService,

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

func (s *NotificationService) sendDirectNotification(ctx context.Context, payload dto.SendNotificationPayload) (*dto.Notification, error) {
	exists, err := s.recipientRepo.Exists(ctx, payload.ProjectID, *payload.RecipientExtID)
	if err != nil {
		return nil, fmt.Errorf("check recipient existence: %w", err)
	}

	// If recipient does not exist, create it.
	// This is to ensure that we can send notifications to recipients that are not yet created.
	if !exists {
		_, _, err := s.recipientService.Create(ctx, dto.CreateRecipientPayload{
			ProjectID:  payload.ProjectID,
			ExternalID: *payload.RecipientExtID,
		})

		if err != nil {
			return nil, fmt.Errorf("create recipient: %w", err)
		}
	}

	notification := entity.NewNotification(
		payload.ProjectID,
		*payload.RecipientExtID,
		payload.Payload,
		nil,
		payload.Target.Channel,
		payload.Target.Topic,
		payload.Target.Event,
	)

	shouldDeliver, err := s.preferenceRepo.ShouldDirectNotificationBeDelivered(ctx, notification.ProjectID, notification.RecipientExtID, dto.TargetFromNotification(notification))
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
		payload.Target.Channel,
		payload.Target.Topic,
		payload.Target.Event,
	)

	broadcast, err := s.broadcastRepo.Create(ctx, broadcast)
	if err != nil {
		return nil, fmt.Errorf("create broadcast: %w", err)
	}

	taskPayload, err := json.Marshal(dto.PrepareBroadcastBatchesPayload{Broadcast: broadcast})
	if err != nil {
		return nil, fmt.Errorf("marshal prepare broadcast batches payload: %w", err)
	}

	task := asynq.NewTask(jobs.TaskTypePrepareBroadcastBatches, taskPayload)

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

func (s *NotificationService) ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*dto.NotificationListItem, *query.Cursor, service.Error, error) {
	if recipientExtID == "" {
		return nil, nil, service.ErrInvalidInput, fmt.Errorf("recipient id required")
	}

	err := cursor.Validate(100, 10)
	if err != nil {
		return nil, nil, service.ErrInvalidInput, err
	}

	fmt.Println("Cusor in service:", cursor)

	notifs, returnedCursor, err := s.repo.ListForRecipient(ctx, projectID, recipientExtID, cursor)
	if err != nil {
		return nil, nil, service.ErrInternalServerError, err
	}

	return dto.FromNotificationList(notifs), returnedCursor, service.ErrNone, nil
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

func (s *NotificationService) MarkAsReadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, service.Error, error) {
	if len(notificationIDs) == 0 {
		return 0, service.ErrInvalidInput, fmt.Errorf("no notification ids provided")
	}

	updated, err := s.repo.MarkAsReadForRecipient(ctx, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) MarkAllAsReadForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, service.Error, error) {
	updated, err := s.repo.MarkAsReadForRecipient(ctx, projectID, recipientExtID, nil)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}
	return updated, service.ErrNone, nil
}

func (s *NotificationService) MarkAsUnreadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, service.Error, error) {
	if len(notificationIDs) == 0 {
		return 0, service.ErrInvalidInput, fmt.Errorf("no notification ids provided")
	}

	updated, err := s.repo.MarkAsUnreadForRecipient(ctx, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) MarkAsOpenedForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, service.Error, error) {
	if len(notificationIDs) == 0 {
		return 0, service.ErrInvalidInput, fmt.Errorf("no notification ids provided")
	}

	updated, err := s.repo.MarkAsOpenedForRecipient(ctx, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}

	return updated, service.ErrNone, nil
}

func (s *NotificationService) MarkAllAsOpenedForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, service.Error, error) {
	updated, err := s.repo.MarkAsOpenedForRecipient(ctx, projectID, recipientExtID, nil)
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
