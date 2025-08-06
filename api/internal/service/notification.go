package service

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type NotificationService struct {
	repo          repository.NotificationRepository
	recipientRepo repository.RecipientRepository
	// broadcastRepo repository.BroadcastRepository
}

func NewNotificationService(repo repository.NotificationRepository, recipientRepo repository.RecipientRepository) *NotificationService {
	return &NotificationService{
		repo:          repo,
		recipientRepo: recipientRepo,
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
			return nil, service.ErrInternalServerError, err
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
			return nil, service.ErrInternalServerError, err
		}
	}

	return result, service.ErrNone, nil
}

func (s *NotificationService) sendTargetedNotification(ctx context.Context, notification *entity.Notification) (*dto.Notification, error) {
	return nil, nil
}

func (s *NotificationService) sendBroadcastNotification(ctx context.Context, broadcast *entity.Broadcast) (*dto.Broadcast, error) {
	return nil, nil
}
