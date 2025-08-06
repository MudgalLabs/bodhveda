package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type NotificationService struct {
	repo           repository.NotificationRepository
	recipientRepo  repository.RecipientRepository
	preferenceRepo repository.PreferenceRepository
	// broadcastRepo repository.BroadcastRepository
}

func NewNotificationService(repo repository.NotificationRepository, recipientRepo repository.RecipientRepository, preferenceRepo repository.PreferenceRepository) *NotificationService {
	return &NotificationService{
		repo:           repo,
		recipientRepo:  recipientRepo,
		preferenceRepo: preferenceRepo,
	}
}

func (s *NotificationService) Send(ctx context.Context, payload dto.SendNotificationPayload) (*dto.SendNotificationResult, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	result := &dto.SendNotificationResult{}

	if payload.IsTargeted() {
		notification := entity.NewNotification(
			payload.ProjectID,
			*payload.To.RecipientExtID,
			payload.Payload,
			nil,
			payload.To.Channel,
			payload.To.Topic,
			payload.To.Event,
		)

		result.Notification, err = s.sendTargetedNotification(ctx, notification)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("send targeted notification: %w", err)
		}
	} else {
		broadcast := entity.NewBroadcast(
			payload.ProjectID,
			payload.Payload,
			payload.To.Channel,
			payload.To.Topic,
			payload.To.Event,
		)

		result.Broadcast, err = s.sendBroadcastNotification(ctx, broadcast)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("send broadcast notification: %w", err)
		}
	}

	return result, service.ErrNone, nil
}

func (s *NotificationService) sendTargetedNotification(ctx context.Context, notification *entity.Notification) (*dto.Notification, error) {
	shouldDeliver, err := s.preferenceRepo.ShouldTagetedNotificationBeDelivered(ctx, notification)
	if err != nil {
		return nil, err
	}

	if !shouldDeliver {
		// Can return nil here, as the notification is not delivered.
		// The result will have nil `notification` field in `SendNotificationResult`.
		return nil, nil
	}

	notification, err = s.repo.Create(ctx, notification)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	return dto.FromNotification(notification), nil
}

func (s *NotificationService) sendBroadcastNotification(ctx context.Context, broadcast *entity.Broadcast) (*dto.Broadcast, error) {
	return nil, nil
}
