package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/job"
	"github.com/mudgallabs/bodhveda/internal/job/processor"
	"github.com/mudgallabs/bodhveda/internal/job/task"
	"github.com/mudgallabs/tantra/logger"
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

	err = run(asynqServer, asynqMux)
	if err != nil {
		panic(err)
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
