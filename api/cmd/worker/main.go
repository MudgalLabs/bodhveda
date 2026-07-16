package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/job"
	"github.com/mudgallabs/bodhveda/internal/job/processor"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/logger"
)

const (
	// webhookEventRetention is how long processed-webhook idempotency rows are kept
	// before the cleanup job prunes them. Only needs to exceed the provider's retry
	// horizon (Svix retries span ~a day) for dedup correctness; the rest is audit
	// headroom.
	webhookEventRetention = 30 * 24 * time.Hour
	// webhookEventCleanupInterval is how often the cleanup job runs.
	webhookEventCleanupInterval = 24 * time.Hour
)

func main() {
	app.Init()
	defer app.Close()

	asynqServer, err := job.NewAsynqServer()
	if err != nil {
		logger.Get().Errorf("failed to create Asynq server: %v", err)
		panic(err)
	}

	asynqMux := asynq.NewServeMux()

	asynqMux.Handle(task.TaskTypeNotificationDelivery, processor.NewNotificationDeliveryProcessor(
		app.DB, app.APP.Repository.Notification, app.APP.Repository.Preference, app.APP.Service.Billing,
	))

	asynqMux.Handle(task.TaskTypeEmailDelivery, processor.NewEmailDeliveryProcessor(
		app.APP.Repository.NotificationDelivery, app.APP.Repository.ProjectEmail,
	))

	asynqMux.Handle(task.TaskTypePrepareBroadcastBatches, processor.NewPrepareBroadcastBatchesProcessor(
		app.DB, app.ASYNQCLIENT, app.APP.Repository.Preference, app.APP.Repository.Broadcast,
		app.APP.Repository.BroadcastBatch, app.APP.Service.Billing,
	))

	asynqMux.Handle(task.TaskTypeBroadcastDelivery, processor.NewBroadcastDeliveryProcessor(
		app.DB, app.APP.Repository.Notification, app.APP.Repository.Broadcast, app.APP.Repository.BroadcastBatch,
	))

	asynqMux.Handle(task.TaskTypeDeleteRecipientData, processor.NewDeleteRecipientDataProcessor(
		app.APP.Repository.Preference, app.APP.Repository.Notification,
		app.APP.Repository.Recipient,
	))

	asynqMux.Handle(task.TaskTypeDeleteProjectData, processor.NewDeleteProjectDataProcessor(
		app.APP.Repository.APIKey, app.APP.Repository.Broadcast, app.APP.Repository.BroadcastBatch,
		app.APP.Repository.Notification, app.APP.Repository.Preference, app.APP.Repository.Project,
		app.APP.Repository.Recipient,
	))

	// Retention cleanup for the webhook idempotency ledger (#8). A lightweight
	// ticker is enough here — a single worker, and DELETE is idempotent — so we
	// avoid standing up an Asynq scheduler for one periodic job. Cancelled when run()
	// returns (graceful shutdown).
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go runWebhookEventCleanup(cleanupCtx, app.APP.Repository.WebhookEvent)

	err = run(asynqServer, asynqMux)
	if err != nil {
		panic(err)
	}
}

// runWebhookEventCleanup prunes webhook_event rows older than the retention window,
// once on start and then on a daily tick, until ctx is cancelled.
func runWebhookEventCleanup(ctx context.Context, repo repository.WebhookEventRepository) {
	l := logger.Get()

	cleanup := func() {
		deleted, err := repo.DeleteOlderThan(ctx, time.Now().Add(-webhookEventRetention))
		if err != nil {
			l.Errorf("webhook_event cleanup: %v", err)
			return
		}
		if deleted > 0 {
			l.Infof("webhook_event cleanup: pruned %d rows older than %s", deleted, webhookEventRetention)
		}
	}

	cleanup()

	ticker := time.NewTicker(webhookEventCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func run(asynqServer *asynq.Server, asynqMux *asynq.ServeMux) error {
	l := logger.Get()

	shutdown := make(chan error)

	// Start Asynq server
	go func() {
		l.Infow("Asynq server started")
		if err := asynqServer.Run(asynqMux); err != nil {
			shutdown <- err
		}
	}()

	// Listen for termination signals
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		l.Infow("Signal caught, initiating shutdown", "signal", s.String())

		// Stop processing tasks gracefully
		asynqServer.Shutdown()

		shutdown <- nil
	}()

	// Block until shutdown completes
	err := <-shutdown
	if err != nil {
		l.Errorw("Shutdown with error", "error", err)
		return err
	}

	l.Infow("Graceful shutdown complete")
	return nil
}
