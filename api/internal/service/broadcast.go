package service

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type BroadcastService struct {
	repo repository.BroadcastRepository
}

func NewBroadcastService(repo repository.BroadcastRepository) *BroadcastService {
	return &BroadcastService{
		repo: repo,
	}
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
