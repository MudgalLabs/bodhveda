package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type BillingService struct {
	db                 *pgxpool.Pool
	projectRepo        repository.ProjectRepository
	subRepo            repository.UserSubscriptionRepository
	usageLogRepo       repository.UsageLogRepository
	usageAggregateRepo repository.UsageAggregateRepository
}

func NewBillingService(
	db *pgxpool.Pool, projectRepo repository.ProjectRepository, subRepo repository.UserSubscriptionRepository,
	usageLogRepo repository.UsageLogRepository, usageAggregateRepo repository.UsageAggregateRepository,
) *BillingService {
	return &BillingService{
		db:                 db,
		projectRepo:        projectRepo,
		subRepo:            subRepo,
		usageLogRepo:       usageLogRepo,
		usageAggregateRepo: usageAggregateRepo,
	}
}

func (s *BillingService) GetSubscription(ctx context.Context, userID int) (*dto.UserSubscription, service.Error, error) {
	sub, err := s.subRepo.Get(ctx, userID)

	if err != nil {
		if err == tantraRepo.ErrNotFound {
			// Create a new free subscription if not found.
			// This is to ensure that every user has at least a free plan.
			sub = entity.NewUserSubscription(userID, entity.PlanFree)
			err = s.subRepo.Upsert(ctx, sub)
			if err != nil {
				return nil, service.ErrInternalServerError, fmt.Errorf("failed to create subscription: %w", err)
			}
		} else {
			return nil, service.ErrInternalServerError, fmt.Errorf("failed to get subscription: %w", err)
		}
	}

	return dto.FromUserSubscription(sub), service.ErrNone, nil
}

func (s *BillingService) GetUsage(ctx context.Context, userID int, planID entity.PlanID, periodStart time.Time, periodEnd time.Time) (map[entity.Metric]dto.UsageAggregate, service.Error, error) {
	projects, err := s.projectRepo.List(ctx, userID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("failed to list projects: %w", err)
	}

	projectIDs := make([]int, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}

	notificationsUsed, err := s.usageAggregateRepo.Get(ctx, projectIDs, entity.MetricNotifications, periodStart, periodEnd)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("failed to get usage for notifications: %w", err)
	}

	plan, ok := entity.GetPlan(planID)
	if !ok {
		return nil, service.ErrBadRequest, fmt.Errorf("unknown plan ID: %s", planID)
	}

	usageMap := map[entity.Metric]dto.UsageAggregate{
		entity.MetricNotifications: dto.NewUsageAggregate(userID, entity.MetricNotifications, notificationsUsed, int64(*plan.Entitlements[entity.MetricNotifications].Limit)),
	}

	return usageMap, service.ErrNone, nil
}

func (s *BillingService) CheckAndConsumeUsage(ctx context.Context, event dto.UsageEvent) error {
	now := time.Now().UTC()

	err := dbx.WithTx(ctx, s.db, func(tx pgx.Tx) error {
		// Load subscription
		sub, _, err := s.GetSubscription(ctx, event.UserID)
		if err != nil {
			return fmt.Errorf("failed to get subscription: %w", err)
		}

		// Plan has expired.
		if now.After(sub.CurrentPeriodEnd) {
			var newSub *entity.UserSubscription

			// If the current/last plan was free, we can just renew it.
			// If the user was on a paid plan, we wait for the grace period before downgrading to free plan.
			if sub.PlanID == entity.PlanFree ||
				now.After(sub.CurrentPeriodEnd.Add(entity.SubscriptionRenewalGracePeriod)) {
				newSub = entity.RenewSubscription(sub.UserID, entity.PlanFree, sub.CreatedAt)
			}

			err = s.subRepo.Upsert(ctx, newSub)
			if err != nil {
				return fmt.Errorf("failed to renew subscription: %w", err)
			}

			sub = dto.FromUserSubscription(newSub)
		}

		// Get plan from hardcoded definitions
		plan, ok := entity.GetPlan(sub.PlanID)
		if !ok {
			return fmt.Errorf("unknown plan ID: %s", sub.PlanID)
		}

		entitlement, ok := plan.Entitlements[event.Metric]
		if !ok {
			return errors.New("metric not available in plan")
		}

		projects, err := s.projectRepo.List(ctx, event.UserID)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		projectIDs := make([]int, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}

		// Check aggregate usage
		used, err := s.usageAggregateRepo.Get(ctx, projectIDs, event.Metric, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
		if err != nil {
			return err
		}

		// If there's a limit, check if the new usage would exceed it
		if entitlement.Limit != nil && used+event.Amount > *entitlement.Limit {
			return enum.ErrQuotaExceeded
		}

		// Record usage
		if err := s.usageLogRepo.Add(ctx, tx, event.ProjectID, event.Metric, event.Amount, sub.CurrentPeriodStart, sub.CurrentPeriodEnd); err != nil {
			return err
		}

		return nil
	})

	return err
}
