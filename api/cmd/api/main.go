package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/tantra/logger"
)

func main() {
	app.Init()
	defer app.Close()

	router := initRouter()

	err := run(router)
	if err != nil {
		panic(err)
	}
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
