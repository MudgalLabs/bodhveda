package main

import (
	"bodhveda/internal/env"
	"bodhveda/internal/session"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

func initRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(attachLoggerMiddleware)
	r.Use(timezoneMiddleware)
	r.Use(logRequestMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{env.WebURL},
		AllowedMethods:   []string{"GET", "DELETE", "OPTIONS", "PATCH", "POST", "PUT"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Timezone"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(session.Manager.LoadAndSave)

	// This is just to prevent abuse of the API by limiting the number of requests
	// from a single IP address. The limit is set to 100 requests per minute.
	// We would never hit this limit in normal usage, but it is a good practice to have
	// this in place to prevent abuse.
	r.Use(httprate.LimitByIP(100, time.Minute))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		successResponse(w, r, http.StatusOK, "Hi, welcome to Bodhveda API. Don't be naughty!", nil)
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		successResponse(w, r, http.StatusOK, "Pong", nil)
	})

	r.Route("/v1", func(r chi.Router) {
		// Core routes that power the Notification service.
		r.Use(apiKeyMiddleware)

		r.Route("/broadcasts", func(r chi.Router) {
			r.Post("/", sendBroadcastHandler(app))
			r.Get("/", fetchBroadcastsHandler(app))
			r.Delete("/", deleteBroadcastsHandler(app))
			r.Delete("/all", deleteAllBroadcastsHandler(app))
		})

		r.Route("/recipients/{recipient}", func(r chi.Router) {
			r.Route("/notifications", func(r chi.Router) {
				r.Post("/", sendNotificationHandler(app))
				r.Get("/", fetchNotificationsHandler(app))
				r.Get("/unread-count", fetchUnreadCountHandler(app))
				r.Post("/read", markNotificationsReadHandler(app))
				r.Post("/read/all", markAllNotificationsReadHandler(app))
				r.Delete("/", deleteNotificationsHandler(app))
				r.Delete("/all", deleteAllNotificationsHandler(app))
			})
		})
	})

	// Platform routes that power the web app.
	r.Route("/v1/platform", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/oauth/google", googleSignInHandler(app.service.UserIdentityService))
			r.Get("/oauth/google/callback", googleCallbackHandler(app.service.UserIdentityService))
			r.Post("/sign-out", signOutHandler(app.service.UserIdentityService))
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(authMiddleware)

			r.Get("/me", getMeHandler(app.service.UserProfileService))
		})
	})

	return r
}
