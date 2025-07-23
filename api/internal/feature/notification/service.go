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

type DirectPayload struct {
	Recipient string          `json:"recipient"`
	Payload   json.RawMessage `json:"payload"`
}

func (s *Service) Direct(ctx context.Context, projectID uuid.UUID, payload *DirectPayload) (*Notification, service.Error, error) {
	plan := project.GetPlan(project.PlanFree)

	if len(payload.Payload) > domain.MaxPayloadSize {
		return nil, service.ErrBadRequest, fmt.Errorf("payload size exceeds maximum limit of 12KB")
	}

	newNotification, err := new(projectID, payload.Recipient, payload.Payload, plan.NotificationExpiresAt())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("new notification: %w", err)
	}

	if err := s.notificationRepository.Create(ctx, newNotification); err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create notification: %w", err)
	}

	return newNotification, service.ErrNone, nil
}

type BroadcastPayload struct {
	Payload json.RawMessage `json:"payload"`
}

func (s *Service) Broadcast(ctx context.Context, projectID uuid.UUID, payload *BroadcastPayload) (*broadcast.Broadcast, service.Error, error) {
	plan := project.GetPlan(project.PlanFree)

	if len(payload.Payload) > domain.MaxPayloadSize {
		return nil, service.ErrBadRequest, fmt.Errorf("payload size exceeds maximum limit of 12KB")
	}

	newBroadcast, err := broadcast.New(projectID, payload.Payload, plan.NotificationExpiresAt())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("new broadcast: %w", err)
	}

	if err := s.broadcastRepository.Create(ctx, newBroadcast); err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create broadcast: %w", err)
	}

	return newBroadcast, service.ErrNone, nil
}
