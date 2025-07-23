package notification

import (
	"bodhveda/internal/domain"
	"bodhveda/internal/feature/project"
	"bodhveda/internal/service"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	notificationRepository ReadWriter
}

func NewService(notificationRepository ReadWriter) *Service {
	return &Service{
		notificationRepository: notificationRepository,
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

	return nil, service.ErrNone, nil
}
