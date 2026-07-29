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
	"github.com/mudgallabs/bodhveda/internal/monitor"
	"github.com/mudgallabs/tantra/logger"
)

func main() {
	app.Init()
	defer app.Close()

	router := initRouter()

	// Infra monitor (internal/monitor). Runs HERE, on the API, and not on the
	// worker: a monitor that rides the same Asynq queue it watches dies in the
	// exact incident it exists to report. Cancelled on graceful shutdown.
	monitorCtx, stopMonitor := context.WithCancel(context.Background())
	defer stopMonitor()
	startMonitor(monitorCtx)

	err := run(router)
	if err != nil {
		panic(err)
	}
}

// startMonitor wires up and launches the infra monitor. A failure to build the
// Asynq inspector is logged and skipped rather than fatal — the API serving
// traffic matters more than its own monitoring, and a hard failure here would
// mean a monitoring bug could take the whole product down.
func startMonitor(ctx context.Context) {
	l := logger.Get()

	redisConnOpt, err := asynq.ParseRedisURI(env.RedisURL)
	if err != nil {
		l.Errorw("monitor: failed to parse Redis URL, infra monitoring is DISABLED", "error", err)
		return
	}

	inspector := asynq.NewInspector(redisConnOpt)

	m := monitor.New(monitor.Config{
		Checks: monitor.DefaultChecks(inspector, app.APP.Repository.Notification, nil),
		Sink:   monitor.NewDiscordSink(env.AlertDiscordWebhookURL),
	})

	go m.Run(ctx)
}

func run(router http.Handler) error {
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
