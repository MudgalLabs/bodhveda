package notification

import (
	"bodhveda/internal/domain"
	"bodhveda/internal/feature/broadcast"
	"bodhveda/internal/feature/project"
	"bodhveda/internal/service"
	"context"
	"encoding/json"
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

// Inbox retrieves the notifications for a recipient in a paginated manner.
func (s *Service) Inbox(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) (*Inbox, service.Error, error) {
	// First of all, we must materialize broadcasts into notifications that aren't already present.

	notifications, total, err := s.notificationRepository.Inbox(ctx, projectID, recipient, limit, offset)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("fetch inbox: %w", err)
	}

	inbox := NewInbox(notifications, total)
	return inbox, service.ErrNone, nil
}
