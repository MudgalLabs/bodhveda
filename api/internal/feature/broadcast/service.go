package broadcast

import (
	"bodhveda/internal/domain"
	"bodhveda/internal/feature/project"
	"bodhveda/internal/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	broadcastRepository ReadWriter
}

func NewService(broadcastRepository ReadWriter) *Service {
	return &Service{
		broadcastRepository: broadcastRepository,
	}
}

func (s *Service) Send(ctx context.Context, projectID uuid.UUID, payload json.RawMessage) (*Broadcast, service.Error, error) {
	plan := project.GetPlan(project.PlanFree)

	if len(payload) > domain.MaxPayloadSize {
		return nil, service.ErrBadRequest, fmt.Errorf("payload size exceeds maximum limit of 16KB")
	}

	newBroadcast, err := New(projectID, payload, plan.NotificationExpiresAt())
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("new broadcast: %w", err)
	}

	if err := s.broadcastRepository.Create(ctx, newBroadcast); err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create broadcast: %w", err)
	}

	return newBroadcast, service.ErrNone, nil
}

// List retrieves the broadcasts for a recipient in a paginated manner.
func (s *Service) List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Broadcast, int, service.Error, error) {
	notifications, total, err := s.broadcastRepository.List(ctx, projectID, limit, offset)
	if err != nil {
		return nil, 0, service.ErrInternalServerError, fmt.Errorf("fetch list: %w", err)
	}

	return notifications, total, service.ErrNone, nil
}

func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) (service.Error, error) {
	if len(ids) == 0 {
		return service.ErrBadRequest, errors.New("no ids provided")
	}
	err := s.broadcastRepository.Delete(ctx, projectID, ids)
	if err != nil {
		return service.ErrInternalServerError, err
	}
	return service.ErrNone, nil
}

func (s *Service) DeleteAll(ctx context.Context, projectID uuid.UUID) (int, service.Error, error) {
	count, err := s.broadcastRepository.DeleteAll(ctx, projectID)
	if err != nil {
		return 0, service.ErrInternalServerError, err
	}
	return count, service.ErrNone, nil
}
