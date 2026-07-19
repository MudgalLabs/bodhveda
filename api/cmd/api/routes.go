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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.SuccessResponse(w, r, http.StatusOK, "Hi, welcome to Bodhveda API. Don't be naughty!", nil)
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		httpx.SuccessResponse(w, r, http.StatusOK, "Pong", nil)
	})

	// Public provider webhook ingestion (Phase 5). Mounted at the root — OUTSIDE
	// the developer API-key auth/CORS/rate-limit group and the console session
	// group — because it is called by the email provider (Resend via Svix), not by
	// a customer: authentication IS the webhook signature, verified in the service
	// against the project's stored signing secret. It is NOT covered by the per-IP
	// dev-API limiter; instead it gets its own loose limiter keyed by PROJECT (not
	// IP) — provider webhooks come from a small shared egress IP pool, so a per-IP
	// limit would throttle every project together. This is a coarse abuse ceiling,
	// well above real per-project event volume; dropped events are retried by Svix.
	r.With(httprate.Limit(
		3000, time.Minute,
		httprate.WithKeyFuncs(webhookRateKey),
	)).Post("/webhooks/email/{project_id}", handler.EmailWebhook(app.APP.Service.EmailWebhook))

	// Public one-click email unsubscribe (Phase 6). Mounted at the root — like the
	// webhook above, OUTSIDE the developer API-key auth/CORS/rate-limit group and the
	// console session group — because it is hit from the recipient's mail client with
	// no session/API key: the signed token in `?t=` IS the auth (it identifies
	// project + recipient + target). POST = RFC 8058 one-click; GET = confirmation
	// page. GET is side-effect-free; the POST flips the recipient's email preference
	// for that target off. Loose per-IP limiter to blunt token-guessing floods
	// (verification is cheap, so the ceiling can be generous); GET + POST share it.
	r.With(httprate.LimitByIP(120, time.Minute)).Group(func(r chi.Router) {
		r.Get("/unsubscribe/email", handler.UnsubscribeEmail(app.APP.Service.Unsubscribe))
		r.Post("/unsubscribe/email", handler.UnsubscribeEmail(app.APP.Service.Unsubscribe))
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

		// Rate limit the developer API to 100 req/min/IP to prevent abuse. Scoped
		// to this group (not the root router) so the public provider webhook is not
		// caught by it — see the /webhooks/email mount above.
		r.Use(httprate.LimitByIP(100, time.Minute))

		r.Use(middleware.APIKeyBasedAuthMiddleware)

		r.Route("/notifications", func(r chi.Router) {
			r.Use(middleware.VerifyAPIKeyHasFullScope)

			r.Post("/send", handler.SendNotification(app.APP.Service.Notification))
			// Read-by-id: the send is fully async (returns a notification id after
			// one INSERT), so callers poll this to learn the resolved in-app status
			// and the email delivery outcome. Mirrors Resend's GET /emails/{id}.
			r.Get("/{notification_id}", handler.GetNotification(app.APP.Service.Notification))
		})

		// Project preference (catalog) CRUD. Full-scope only — the catalog
		// defines what a whole project may send, so a recipient-scoped key has no
		// business touching it. Project-scoped by the API key (no project_id in
		// the path), mirroring the rest of the Developer API. The console keeps
		// its own /console project_id-in-path preference routes unchanged.
		r.Route("/preferences", func(r chi.Router) {
			r.Use(middleware.VerifyAPIKeyHasFullScope)

			r.Get("/", handler.ListProjectPreferencesAPI(app.APP.Service.Preference))
			r.Post("/", handler.CreateProjectPreferenceAPI(app.APP.Service.Preference))
			// Declarative bulk merge of the whole catalog (array body). ?prune=true
			// also removes catalog rows absent from the array; default is merge.
			r.Put("/", handler.UpsertProjectPreferencesAPI(app.APP.Service.Preference))

			r.Route("/{preference_id}", func(r chi.Router) {
				r.Get("/", handler.GetProjectPreferenceAPI(app.APP.Service.Preference))
				r.Patch("/", handler.UpdateProjectPreferenceAPI(app.APP.Service.Preference))
				r.Delete("/", handler.DeleteProjectPreferenceAPI(app.APP.Service.Preference))
			})
		})

		r.Route("/recipients", func(r chi.Router) {
			r.With(middleware.VerifyAPIKeyHasFullScope).Group(func(r chi.Router) {
				r.Post("/", handler.CreateRecipient(app.APP.Service.Recipient))
				r.Post("/batch", handler.BatchCreateRecipients(app.APP.Service.Recipient))
			})

			r.Route("/{recipient_external_id}", func(r chi.Router) {
				r.Use(middleware.CreateRecipientIfNotExists)

				r.With(middleware.VerifyAPIKeyHasFullScope).Group(func(r chi.Router) {
					r.Get("/", handler.GetRecipient(app.APP.Service.Recipient))
					r.Patch("/", handler.UpdateRecipient(app.APP.Service.Recipient))
					r.Delete("/", handler.DeleteRecipient(app.APP.Service.Recipient))
				})

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

				r.Route("/contacts", func(r chi.Router) {
					// POST/PUT/GET/PATCH are allowed for full OR recipient-scoped keys
					// (like preferences). DELETE has the highest blast radius on a
					// stolen recipient key, so it requires full scope.
					r.Post("/", handler.CreateRecipientContact(app.APP.Service.RecipientContact))
					// PUT = idempotent "ensure this is the primary contact for this
					// medium" (create-or-update). Lets a server sync be one call.
					r.Put("/", handler.SetPrimaryRecipientContact(app.APP.Service.RecipientContact))
					r.Get("/", handler.ListRecipientContacts(app.APP.Service.RecipientContact))

					r.Route("/{contact_id}", func(r chi.Router) {
						r.Patch("/", handler.UpdateRecipientContact(app.APP.Service.RecipientContact))
						r.With(middleware.VerifyAPIKeyHasFullScope).Delete("/", handler.DeleteRecipientContact(app.APP.Service.RecipientContact))
					})
				})
			})
		})
	})

	// These are the APIs that power the Bodhveda Console.
	r.Route("/console", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{env.WebURL},
			AllowedMethods: []string{"GET", "DELETE", "OPTIONS", "PATCH", "POST", "PUT"},
			// X-Timezone lets the console send the viewer's IANA timezone so the
			// analytics endpoint (Phase 9.5) buckets per-day in it. It is a
			// non-simple header, so without it here the browser's preflight fails
			// (curl doesn't preflight, which is why only browser-driving caught it).
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Timezone"},
			AllowCredentials: true,
			ExposedHeaders:   []string{"*"},
			MaxAge:           300,
		}))

		// Same per-IP abuse limit the console had when this lived on the root router.
		r.Use(httprate.LimitByIP(100, time.Minute))

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

				r.Patch("/", handler.UpdateProject(app.APP.Service.Project))
				r.Delete("/", handler.DeleteProject(app.APP.Service.Project))

				r.Route("/api-keys", func(r chi.Router) {
					r.Get("/", handler.ListAPIKeys(app.APP.Service.APIKey))
					r.Post("/", handler.CreateAPIKey(app.APP.Service.APIKey))
					r.Delete("/{api_key_id}", handler.DeleteAPIKey(app.APP.Service.APIKey))
				})

				r.Route("/broadcasts", func(r chi.Router) {
					r.Get("/", handler.ListBroadcasts(app.APP.Service.Broadcast))
				})

				r.Route("/email-settings", func(r chi.Router) {
					r.Get("/", handler.GetProjectEmailSettings(app.APP.Service.ProjectEmail))
					r.Put("/", handler.UpsertProjectEmailSettings(app.APP.Service.ProjectEmail))
				})

				r.Route("/notifications", func(r chi.Router) {
					r.Get("/", handler.List(app.APP.Service.Notification))
					r.Post("/send", handler.SendNotificationConsole(app.APP.Service.Notification))
					r.Get("/{notification_id}/deliveries", handler.ListNotificationDeliveries(app.APP.Service.Notification))
				})

				r.Get("/analytics", handler.ProjectAnalytics(app.APP.Service.Notification))

				r.Route("/preferences", func(r chi.Router) {
					r.Get("/", handler.ListPreferences(app.APP.Service.Preference))
					r.Post("/", handler.CreateProjectPreference(app.APP.Service.Preference))

					r.Route("/{preference_id}", func(r chi.Router) {
						r.Patch("/", handler.UpdateProjectPreference(app.APP.Service.Preference))
						r.Delete("/", handler.DeletePreference(app.APP.Service.Preference))
					})
				})

				r.Route("/recipients", func(r chi.Router) {
					r.Get("/", handler.ListRecipients(app.APP.Service.Recipient))
					r.Post("/", handler.CreateRecipientConsole(app.APP.Service.Recipient))

					r.Route("/{recipient_external_id}", func(r chi.Router) {
						r.Get("/", handler.GetRecipientConsole(app.APP.Service.Recipient))
						r.Patch("/", handler.UpdateRecipientConsole(app.APP.Service.Recipient))
						r.Delete("/", handler.DeleteRecipientConsole(app.APP.Service.Recipient))
						r.Get("/preferences", handler.GetRecipientPreferencesConsole(app.APP.Service.Preference))
						r.Put("/preferences", handler.UpsertRecipientPreferences(app.APP.Service.Preference))

						r.Route("/contacts", func(r chi.Router) {
							r.Get("/", handler.ListRecipientContactsConsole(app.APP.Service.RecipientContact))
							r.Post("/", handler.CreateRecipientContactConsole(app.APP.Service.RecipientContact))

							r.Route("/{contact_id}", func(r chi.Router) {
								r.Patch("/", handler.UpdateRecipientContactConsole(app.APP.Service.RecipientContact))
								r.Delete("/", handler.DeleteRecipientContactConsole(app.APP.Service.RecipientContact))
							})
						})
					})
				})
			})
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware)

			r.Route("/me", func(r chi.Router) {
				r.Get("/", handler.GetUserMe(app.APP.Service.UserProfile))
				r.Get("/billing", handler.GetUserMeBilling(app.APP.Service.Billing))
			})
		})
	})

	return r
}

// webhookRateKey keys the public provider-webhook limiter by project id (from the
// URL path) rather than by IP. Provider webhooks (Resend via Svix) originate from a
// small, shared egress IP pool, so a per-IP limit would lump every project's
// webhooks into one bucket. Falls back to IP if the path param is somehow absent.
func webhookRateKey(r *http.Request) (string, error) {
	if pid := chi.URLParam(r, "project_id"); pid != "" {
		return "webhook:project:" + pid, nil
	}
	return httprate.KeyByIP(r)
}
