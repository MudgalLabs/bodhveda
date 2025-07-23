package main

import (
	"bodhveda/internal/env"
	"bodhveda/internal/feature/user_identity"
	"bodhveda/internal/logger"
	"bodhveda/internal/oauth"
	"bodhveda/internal/session"
	"net/http"
)

func googleSignInHandler(_ *user_identity.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := oauth.GoogleConfig.AuthCodeURL("")
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

func googleCallbackHandler(s *user_identity.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: We should validate the state parameter here to prevent CSRF attacks.
		// The state parameter should be a random string that is signed and verified.
		// For now, we are skipping this step for simplicity.
		ctx := r.Context()
		l := logger.FromCtx(ctx)

		webURL := env.WebURL

		if webURL == "" {
			panic("WEB_URL is not set in environment variables")
		}

		code := r.URL.Query().Get("code")

		if code == "" {
			l.Error("Missing code parameter in OAuth callback")
			http.Redirect(w, r, webURL+"?oauth_error=true", http.StatusFound)
			return
		}

		userProfile, _, err := s.OAuthGoogleCallback(ctx, code)

		if err != nil {
			l.Errorw("Error during OAuth Google callback", "error", err)
			http.Redirect(w, r, webURL+"?oauth_error=true", http.StatusFound)
			return
		}

		session.Manager.Put(ctx, "user_id", userProfile.UserID.String())
		http.Redirect(w, r, webURL, http.StatusFound)
	}
}

func signOutHandler(_ *user_identity.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session.Manager.Destroy(r.Context())
		successResponse(w, r, http.StatusOK, "Signout successful", nil)
	}
}
