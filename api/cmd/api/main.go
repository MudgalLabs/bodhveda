package main

import (
	"bodhveda/internal/dbx"
	"bodhveda/internal/env"
	"bodhveda/internal/feature/user_identity"
	"bodhveda/internal/feature/user_profile"
	"bodhveda/internal/logger"
	"bodhveda/internal/oauth"
	"bodhveda/internal/session"
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// This contains all the global state that we need to run the API.
// Like all the services and repositories of Arthveda.
type app struct {
	service    services
	repository repositories
}

// All the services.
type services struct {
	UserIdentityService *user_identity.Service
	UserProfileService  *user_profile.Service
}

// Access to all repositories for reading.
// Write access only available to services.
type repositories struct {
	UserIdentity user_identity.Reader
	UserProfile  user_profile.Reader
}

func main() {
	env.Init("../.env")

	// IDK what this does but it was on the blogpost so I'm using it.
	// I think it has something to do with Go sync for multi threading?
	defer logger.Get().Sync()

	session.Init()

	db, err := dbx.Init()
	if err != nil {
		panic(err)
	}

	defer db.Close()

	oauth.InitGoogle()

	userProfileRepository := user_profile.NewRepository(db)
	userIdentityRepository := user_identity.NewRepository(db)

	userIdentityService := user_identity.NewService(userIdentityRepository, userProfileRepository)
	userProfileService := user_profile.NewService(userProfileRepository)

	services := services{
		UserIdentityService: userIdentityService,
		UserProfileService:  userProfileService,
	}

	repositories := repositories{
		UserIdentity: userIdentityRepository,
		UserProfile:  userProfileRepository,
	}

	a := &app{
		service:    services,
		repository: repositories,
	}

	r := initRouter(a)

	err = run(r)
	if err != nil {
		panic(err)
	}
}

func run(router http.Handler) error {
	l := logger.Get()
	srv := &http.Server{
		Addr:         ":1337",
		Handler:      router,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	shutdown := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		l.Infow("signal caught", "signal", s.String())

		shutdown <- srv.Shutdown(ctx)
	}()

	l.Infow("server has started", "addr", srv.Addr, "env", env.APIEnv)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	l.Infow("server has stopped", "addr", srv.Addr, "env", env.APIEnv)
	return nil
}
