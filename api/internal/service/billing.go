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

// CheckAndConsumeUsage gates an event against the user's plan entitlement and,
// if it fits, meters it.
//
// The plan/usage LOOKUPS below deliberately run OUTSIDE the transaction, and must
// stay outside it. dbx.WithTx pins one pool connection for the whole life of the
// transaction, while every repository here issues its query against the pool.
// Running a lookup inside the closure therefore makes a single in-flight call
// hold TWO pool connections at once — and that deadlocks the pool outright:
// pgxpool defaults to max(4, NumCPU) connections (4 on the 2-core VPS) while the
// worker runs Concurrency: 10, so a burst of concurrent sends parks every
// goroutine holding a transaction connection and waiting for a second one that
// only another blocked goroutine could release. pgxpool.Acquire waits until the
// context dies, so the tasks hung for the full 1800s Asynq timeout and the
// delivery queue stopped draining (2026-07-28 incident).
//
// Hoisting them costs nothing in isolation terms: on separate connections they
// never observed the transaction's uncommitted state anyway, so this preserves
// the previous semantics exactly while holding one connection at a time.
//
// KNOWN LIMITATION, unchanged by this: the quota check is check-then-act with no
// row lock, so concurrent sends can overshoot a limit slightly. That race exists
// in the shipped behaviour above too — closing it needs a locking read and is a
// deliberate behaviour change, not part of this fix.
func (s *BillingService) CheckAndConsumeUsage(ctx context.Context, event dto.UsageEvent) error {
	now := time.Now().UTC()

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

	// Record usage. The transaction is scoped to just this write, which is the
	// only part that needs to be atomic: usage_log and usage_aggregate must move
	// together or not at all.
	return dbx.WithTx(ctx, s.db, func(tx pgx.Tx) error {
		return s.usageLogRepo.Add(ctx, tx, event.ProjectID, event.Metric, event.Amount, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	})
}
