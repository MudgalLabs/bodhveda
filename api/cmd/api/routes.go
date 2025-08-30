package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	chimiddleware "github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/handler"
	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/tantra/auth/session"
	"github.com/mudgallabs/tantra/httpx"
)

func initRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.AttachLoggerMiddleware)
	r.Use(middleware.TimezoneMiddleware)
	r.Use(middleware.LogRequestMiddleware)
	r.Use(chimiddleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(chimiddleware.Timeout(60 * time.Second))

	r.Use(session.Manager.LoadAndSave)

	// This is just to prevent abuse of the API by limiting the number of requests
	// from a single IP address. The limit is set to 100 requests per minute.
	// We would never hit this limit in normal usage, but it is a good practice to have
	// this in place to prevent abuse.
	r.Use(httprate.LimitByIP(100, time.Minute))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.SuccessResponse(w, r, http.StatusOK, "Hi, welcome to Bodhveda API. Don't be naughty!", nil)
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		httpx.SuccessResponse(w, r, http.StatusOK, "Pong", nil)
	})

	// These are the Bodhveda Developer API routes.
	r.Route("/", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"}, // Permissive CORS, because these APIs can be called from web frontend apps.
			AllowedMethods:   []string{"GET", "DELETE", "OPTIONS", "PATCH", "POST", "PUT"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Timezone"},
			AllowCredentials: false,
			ExposedHeaders:   []string{"*"},
			MaxAge:           300,
		}))

		r.Use(middleware.APIKeyBasedAuthMiddleware)

		r.Route("/notifications", func(r chi.Router) {
			r.Use(middleware.VerifyAPIKeyHasFullScope)

			r.Post("/send", handler.SendNotification(app.APP.Service.Notification))
		})

		r.Route("/recipients", func(r chi.Router) {
			r.Route("/{recipient_external_id}", func(r chi.Router) {
				r.Use(middleware.CreateRecipientIfNotExists)

				r.Route("/notifications", func(r chi.Router) {
					r.Get("/", handler.ListForRecipient(app.APP.Service.Notification))
					r.Get("/unread-count", handler.UnreadCountForRecipient(app.APP.Service.Notification))
					r.Patch("/", handler.UpdateRecipientNotifications(app.APP.Service.Notification))
					r.Delete("/", handler.DeleteRecipientNotifications(app.APP.Service.Notification))
				})

				r.Route("/preferences", func(r chi.Router) {
					r.Get("/", handler.GetRecipientProjectPreferences(app.APP.Service.Preference))
					r.Patch("/", handler.UpdateRecipientPreferenceForTarget(app.APP.Service.Preference))
					r.Get("/check", handler.CheckRecipientPreferenceForTarget(app.APP.Service.Preference))
				})
			})

			r.Route("/", func(r chi.Router) {
				r.Use(middleware.VerifyAPIKeyHasFullScope)

				r.Post("/", handler.CreateRecipient(app.APP.Service.Recipient))
				r.Post("/batch", handler.BatchCreateRecipients(app.APP.Service.Recipient))
				r.Get("/", handler.GetRecipient(app.APP.Service.Recipient))
				r.Patch("/", handler.UpdateRecipient(app.APP.Service.Recipient))
				r.Delete("/", handler.DeleteRecipient(app.APP.Service.Recipient))
			})

		})
	})

	// These are the APIs that power the Bodhveda Console.
	r.Route("/console", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{env.WebURL},
			AllowedMethods:   []string{"GET", "DELETE", "OPTIONS", "PATCH", "POST", "PUT"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: true,
			ExposedHeaders:   []string{"*"},
			MaxAge:           300,
		}))

		r.Route("/auth", func(r chi.Router) {
			r.Get("/oauth/google", handler.GoogleSignInHandler(app.APP.Service.UserIdentity))
			r.Get("/oauth/google/callback", handler.GoogleCallbackHandler(app.APP.Service.UserIdentity))
			r.Post("/sign-out", handler.SignOutHandler(app.APP.Service.UserIdentity))
		})

		r.Route("/projects", func(r chi.Router) {
			// Ensure that the user is authenticated before allowing access to the routes.
			r.Use(middleware.AuthMiddleware)

			r.Get("/", handler.ListProjects(app.APP.Service.Project))
			r.Post("/", handler.CreateProject(app.APP.Service.Project))

			r.Route("/{project_id}", func(r chi.Router) {
				// Ensure that the user owns the project before allowing access to the routes.
				r.Use(middleware.VerifyUserOwnsThisProject)

				r.Delete("/", handler.DeleteProject(app.APP.Service.Project))

				r.Route("/api-keys", func(r chi.Router) {
					r.Get("/", handler.ListAPIKeys(app.APP.Service.APIKey))
					r.Post("/", handler.CreateAPIKey(app.APP.Service.APIKey))
					r.Delete("/{api_key_id}", handler.DeleteAPIKey(app.APP.Service.APIKey))
				})

				r.Route("/broadcasts", func(r chi.Router) {
					r.Get("/", handler.ListBroadcasts(app.APP.Service.Broadcast))
				})

				r.Route("/notifications", func(r chi.Router) {
					r.Get("/", handler.List(app.APP.Service.Notification))
					r.Post("/send", handler.SendNotificationConsole(app.APP.Service.Notification))
				})

				r.Route("/preferences", func(r chi.Router) {
					r.Get("/", handler.ListPreferences(app.APP.Service.Preference))
					r.Post("/", handler.CreateProjectPreference(app.APP.Service.Preference))

					r.Route("/{preference_id}", func(r chi.Router) {
						r.Delete("/", handler.DeletePreference(app.APP.Service.Preference))
					})
				})

				r.Route("/recipients", func(r chi.Router) {
					r.Get("/", handler.ListRecipients(app.APP.Service.Recipient))
					r.Post("/", handler.CreateRecipientConsole(app.APP.Service.Recipient))

					r.Route("/{recipient_external_id}", func(r chi.Router) {
						r.Patch("/", handler.UpdateRecipientConsole(app.APP.Service.Recipient))
						r.Delete("/", handler.DeleteRecipientConsole(app.APP.Service.Recipient))
						r.Put("/preferences", handler.UpsertRecipientPreferences(app.APP.Service.Preference))
					})
				})
			})
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware)

			r.Get("/me", handler.GetUserMeHandler(app.APP.Service.UserProfile))
		})
	})

	return r
}
