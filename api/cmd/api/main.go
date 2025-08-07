package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/jobs"
	"github.com/mudgallabs/tantra/logger"
)

func main() {
	app.Init()
	defer app.Close()

	asynqServer, err := jobs.NewAsynqServer()
	if err != nil {
		logger.Get().Errorf("failed to create asynq server: %v", err)
		panic(err)
	}

	asynqMux := asynq.NewServeMux()
	asynqMux.Handle(jobs.TaskTypeBroadcastDelivery, jobs.NewBroadcastProcessor(app.DB, app.APP.Repository.Notification, app.APP.Repository.BroadcastBatch))

	router := initRouter()

	err = run(router, asynqServer, asynqMux)
	if err != nil {
		panic(err)
	}
}

func run(router http.Handler, asynqServer *asynq.Server, asynqMux *asynq.ServeMux) error {
	l := logger.Get()
	httpSrv := &http.Server{
		Addr:         ":1338",
		Handler:      router,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	shutdown := make(chan error)

	// Start HTTP server
	go func() {
		l.Infow("HTTP server started", "addr", httpSrv.Addr, "env", env.APIEnv)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdown <- err
		}
	}()

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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Stop accepting new connections
		if err := httpSrv.Shutdown(ctx); err != nil {
			l.Errorw("Error shutting down HTTP server", "error", err)
		}

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
