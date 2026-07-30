package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type BroadcastService struct {
	repo             repository.BroadcastRepository
	notificationRepo repository.NotificationRepository
}

func NewBroadcastService(repo repository.BroadcastRepository, notificationRepo repository.NotificationRepository) *BroadcastService {
	return &BroadcastService{
		repo:             repo,
		notificationRepo: notificationRepo,
	}
}

// GetBroadcast returns one broadcast by id, scoped to the project.
//
// The detail page needs what the tree does not carry — status, payload,
// timestamps — and keeping them in separate endpoints matches the notifications
// side, where the row and its tree are also fetched separately.
func (s *BroadcastService) GetBroadcast(ctx context.Context, projectID, broadcastID int) (*dto.Broadcast, service.Error, error) {
	broadcast, err := s.repo.GetByID(ctx, broadcastID)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			return nil, service.ErrNotFound, err
		}
		return nil, service.ErrInternalServerError, err
	}

	// Same reasoning as GetDeliveryTree: a broadcast from another project must
	// 404, not leak its existence.
	if broadcast.ProjectID != projectID {
		return nil, service.ErrNotFound, fmt.Errorf("broadcast %d is not in project %d", broadcastID, projectID)
	}

	return dto.FromBroadcast(broadcast), service.ErrNone, nil
}

// GetDeliveryTree returns the per-medium delivery breakdown for one broadcast.
//
// ⚠️ Ownership is enforced HERE, not in the rollup query. The rollup is keyed by
// broadcast_id alone so it stays covered by ix_notification_broadcast, which
// means a broadcast belonging to another project would otherwise return zero
// counts and render as a legitimately empty broadcast. It must 404 instead.
func (s *BroadcastService) GetDeliveryTree(ctx context.Context, projectID, broadcastID int) (*dto.DeliveryTree, service.Error, error) {
	broadcast, err := s.repo.GetByID(ctx, broadcastID)
	if err != nil {
		if errors.Is(err, tantraRepo.ErrNotFound) {
			return nil, service.ErrNotFound, err
		}
		return nil, service.ErrInternalServerError, err
	}

	if broadcast.ProjectID != projectID {
		return nil, service.ErrNotFound, fmt.Errorf("broadcast %d is not in project %d", broadcastID, projectID)
	}

	rollup, err := s.notificationRepo.StatusRollupForBroadcast(ctx, broadcastID)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	tree := &dto.DeliveryTree{
		Kind: enum.NotificationKindBroadcast,
		Target: dto.Target{
			Channel: broadcast.Channel,
			Topic:   broadcast.Topic,
			Event:   broadcast.Event,
		},
		// Broadcasts are in-app only (email is a 400 on broadcast), so there is
		// exactly one branch today. The field is a list so broadcast email needs
		// no contract change when it lands.
		Mediums: []dto.DeliveryTreeMedium{dto.InAppMediumFromRollup(rollup)},
	}

	if a := broadcast.Audience; a != nil {
		tree.Audience = &dto.DeliveryTreeAudience{
			Total:                a.Total,
			Eligible:             a.Eligible,
			ExcludedDisabled:     a.ExcludedDisabled,
			ExcludedNotCataloged: a.ExcludedNotCataloged,
			// Always false for a broadcast: excluded recipients are filtered out
			// before any row is written, so there is nothing to drill into.
			Expandable: false,
		}
	}

	return tree, service.ErrNone, nil
}

func (s *BroadcastService) List(ctx context.Context, payload *dto.ListBroadcastsFilters) (*dto.ListBroadcastssResult, service.Error, error) {
	payload.Pagination.ApplyDefaults()

	broadcasts, total, err := s.repo.List(ctx, payload.ProjectID, payload.Pagination)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	return &dto.ListBroadcastssResult{
		Broadcasts: broadcasts,
		Pagination: payload.Pagination.GetMeta(total),
	}, service.ErrNone, nil
}
