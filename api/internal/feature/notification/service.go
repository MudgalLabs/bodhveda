package notification

import (
	"bodhveda/internal/domain"
	"bodhveda/internal/feature/broadcast"
	"bodhveda/internal/feature/project"
	"bodhveda/internal/logger"
	"bodhveda/internal/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	broadcastRepository    broadcast.ReadWriter
	notificationRepository ReadWriter
}

func NewService(notificationRepository ReadWriter, broadcastRepository broadcast.ReadWriter) *Service {
	return &Service{
		notificationRepository: notificationRepository,
		broadcastRepository:    broadcastRepository,
	}
}

func (s *Service) Direct(ctx context.Context, projectID uuid.UUID, recipient string, payload json.RawMessage) (*Notification, service.Error, error) {
	plan := project.GetPlan(project.PlanFree)

	if len(payload) > domain.MaxPayloadSize {
		return nil, service.ErrBadRequest, fmt.Errorf("payload size exceeds maximum limit of 16KB")
	}

	newNotification, err := new(projectID, recipient, payload, plan.NotificationExpiresAt())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("new notification: %w", err)
	}

	if err := s.notificationRepository.Create(ctx, newNotification); err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create notification: %w", err)
	}

	return newNotification, service.ErrNone, nil
}

func (s *Service) Broadcast(ctx context.Context, projectID uuid.UUID, payload json.RawMessage) (*broadcast.Broadcast, service.Error, error) {
	plan := project.GetPlan(project.PlanFree)

	if len(payload) > domain.MaxPayloadSize {
		return nil, service.ErrBadRequest, fmt.Errorf("payload size exceeds maximum limit of 16KB")
	}

	newBroadcast, err := broadcast.New(projectID, payload, plan.NotificationExpiresAt())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("new broadcast: %w", err)
	}

	if err := s.broadcastRepository.Create(ctx, newBroadcast); err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create broadcast: %w", err)
	}

	return newBroadcast, service.ErrNone, nil
}

// List retrieves the notifications for a recipient in a paginated manner.
func (s *Service) List(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) (*Inbox, service.Error, error) {
	l := logger.FromCtx(ctx)

	// First of all, we must materialize broadcasts into notifications that aren't already present.
	unmaterializedBroadcasts, unmaterializedTotal, err := s.broadcastRepository.Unmaterialized(ctx, projectID, recipient)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("fetch unmaterialized broadcasts: %w", err)
	}

	batchNotifications := make([]*Notification, 0, len(unmaterializedBroadcasts))
	for _, broadcast := range unmaterializedBroadcasts {
		newNotification, err := new(projectID, recipient, broadcast.Payload, broadcast.ExpiresAt)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("new notification from broadcast: %w", err)
		}
		// Associate the broadcast ID with the notification.
		newNotification.BroadcastID = &broadcast.ID
		batchNotifications = append(batchNotifications, newNotification)
	}

	if len(batchNotifications) > 0 {
		l.Infow("Found unmaterialized broadcasts", "count", unmaterializedTotal, "projectID", projectID, "recipient", recipient)

		if err := s.notificationRepository.Materialize(ctx, batchNotifications); err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("batch create notifications: %w", err)
		}

		l.Infow("Materialized broadcasts into notifications", "count", len(batchNotifications), "projectID", projectID, "recipient", recipient)
	}

	notifications, inboxTotal, err := s.notificationRepository.List(ctx, projectID, recipient, limit, offset)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("fetch list: %w", err)
	}

	inbox := NewInbox(notifications, inboxTotal)
	return inbox, service.ErrNone, nil
}

func (s *Service) UnreadCount(ctx context.Context, projectID uuid.UUID, recipient string) (int, service.Error, error) {
	count, err := s.notificationRepository.UnreadCount(ctx, projectID, recipient)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}
	return count, service.ErrNone, nil
}

func (s *Service) MarkAsRead(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) (service.Error, error) {
	if len(ids) == 0 {
		return service.ErrBadRequest, errors.New("no ids provided")
	}

	err := s.notificationRepository.MarkAsRead(ctx, projectID, recipient, ids)
	if err != nil {
		return service.ErrInternalServerError, err
	}

	return service.ErrNone, nil
}

func (s *Service) MarkAllAsRead(ctx context.Context, projectID uuid.UUID, recipient string) (service.Error, error) {
	err := s.notificationRepository.MarkAllAsRead(ctx, projectID, recipient)
	if err != nil {
		return service.ErrInternalServerError, err
	}
	return service.ErrNone, nil
}

func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) (service.Error, error) {
	if len(ids) == 0 {
		return service.ErrBadRequest, errors.New("no ids provided")
	}
	err := s.notificationRepository.Delete(ctx, projectID, recipient, ids)
	if err != nil {
		return service.ErrInternalServerError, err
	}
	return service.ErrNone, nil
}

func (s *Service) DeleteAll(ctx context.Context, projectID uuid.UUID, recipient string) (service.Error, error) {
	err := s.notificationRepository.DeleteAll(ctx, projectID, recipient)
	if err != nil {
		return service.ErrInternalServerError, err
	}
	return service.ErrNone, nil
}
