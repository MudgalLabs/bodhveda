package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/mudgallabs/bodhveda/internal/app"
	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/auth/session"
	"github.com/mudgallabs/tantra/cipher"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/logger"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"go.uber.org/zap"
)

type contextKey string

const ctxUserIDKey contextKey = "user_id"
const ctxUserTimezoneKey contextKey = "user_timezone"
const ctxAPIKey contextKey = "api_key"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromCtx(ctx)
		errorMsg := "You need to be signed in to use this route. POST /auth/sign-in to sign in."

		// Check if the session cookie exists
		_, err := r.Cookie("session")
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				l.Warn("no session found")
				httpx.UnauthorizedResponse(w, r, errorMsg, errors.New("no session found"))
				return
			}
		}

		userID := session.Manager.GetInt(ctx, "user_id")

		if userID == 0 {
			l.Warnw("no user ID found in the session")
			httpx.UnauthorizedResponse(w, r, errorMsg, errors.New("no user ID found in session"))
			return
		}

		l.Debugw("user ID found in the session", "user_id", userID)

		// Extend the session lifetime.
		session.Manager.SetDeadline(ctx, time.Now().Add(session.Lifetime))

		ctx = context.WithValue(ctx, ctxUserIDKey, userID)

		// Add `user_id` to this ctx's logger.
		l = l.With(zap.String(string(ctxUserIDKey), strconv.Itoa(userID)))
		lrw := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// the logger is associated with the request context here
		// so that it may be retrieved in subsequent `http.Handlers`
		r = r.WithContext(logger.WithCtx(ctx, l))

		next.ServeHTTP(lrw, r)
	})
}

func GetUserIDFromContext(ctx context.Context) int {
	id, ok := ctx.Value(ctxUserIDKey).(int)
	if !ok {
		panic("user ID not valid in context")
	}
	return id
}

const requestIDCtxKey = "request_id"

func AttachLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.Get()

		requestID := middleware.GetReqID(ctx)

		// create a child logger containing the request ID so that it appears
		// in all subsequent logs
		l = l.With(zap.String(string(requestIDCtxKey), requestID))

		w.Header().Add(middleware.RequestIDHeader, requestID)

		lrw := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// the logger is associated with the request context here
		// so that it may be retrieved in subsequent `http.Handlers`
		r = r.WithContext(logger.WithCtx(ctx, l))

		next.ServeHTTP(lrw, r)
	})
}

func LogRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromCtx(ctx)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		t1 := time.Now()

		defer func() {
			status := ww.Status()

			reqLogger := l.With(
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.Int("status", status),
				zap.Duration("elapsed", time.Since(t1)),
				zap.String("ip", r.RemoteAddr),
			)

			if status >= 500 && status < 600 {
				reqLogger.Errorf("[ %d SERVER ERROR ]", status)
			} else if status >= 400 && status < 500 {
				reqLogger.Infof("[ %d CLIENT ERROR ]", status)
			} else if status >= 300 && status < 400 {
				reqLogger.Infof("[ %d REDIRECTION ]", status)
			} else if status >= 200 && status < 300 {
				reqLogger.Infof("[ %d SUCCESS ]", status)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}

func TimezoneMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tz := r.Header.Get("X-Timezone")
		tz = strings.TrimSpace(tz)

		if tz == "" {
			tz = "UTC" // fallback
		}

		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}

		ctx := context.WithValue(r.Context(), ctxUserTimezoneKey, loc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserTimezoneFromCtx(ctx context.Context) *time.Location {
	loc, ok := ctx.Value(ctxUserTimezoneKey).(*time.Location)
	if !ok || loc == nil {
		return time.UTC
	}
	return loc
}

// GetAPIKeyFromContext fetches the API key from the context.
func GetAPIKeyFromContext(ctx context.Context) *entity.APIKey {
	apiKey, ok := ctx.Value(ctxAPIKey).(*entity.APIKey)
	if !ok {
		panic("API Key is not valid in context")
	}
	return apiKey
}

func APIKeyBasedAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			httpx.UnauthorizedResponse(w, r, "Missing Authorization header", errors.New("Missing Authorization header"))
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.UnauthorizedResponse(w, r, "Invalid Authorization header format", errors.New("Invalid Authorization header format"))
			return
		}

		tokenPlain := parts[1]
		if tokenPlain == "" {
			httpx.UnauthorizedResponse(w, r, "Missing API Key in Authorization header", errors.New("Missing API Key in Authorization header"))
			return
		}

		tokenHash := cipher.HashToken(tokenPlain, []byte(env.HashKey))

		apiKey, err := app.APP.Repository.APIKey.GetByTokenHash(ctx, tokenHash)
		if err != nil {
			httpx.UnauthorizedResponse(w, r, "Invalid API Key", errors.New("Invalid API Key"))
			return
		}

		ctx = context.WithValue(ctx, ctxAPIKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func VerifyUserOwnsThisProject(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := GetUserIDFromContext(ctx)
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		owns, err := app.APP.Repository.Project.UserOwns(ctx, userID, projectID)
		if err != nil {
			httpx.InternalServerErrorResponse(w, r, errors.New("failed to check project ownership"))
			return
		}

		if !owns {
			// NOTE: Not leaking whether the project exists or not.
			// That's why it's a NotFound error instead of Unauthorized or Forbidden.
			httpx.NotFoundResponse(w, r, errors.New("Project not found"))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func VerifyAPIKeyHasFullScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := GetAPIKeyFromContext(ctx)

		if apiKey.Scope != enum.APIKeyScopeFull {
			httpx.ForbiddenResponse(w, r, "API key does not have sufficient permissions.", errors.New("API key does not have sufficient permissions."))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func VerifyRecipientExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		apiKey := GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.UnauthorizedResponse(w, r, "API key required", errors.New("API key required"))
			return
		}

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		_, err := app.APP.Repository.Recipient.Get(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			if err == tantraRepo.ErrNotFound {
				httpx.NotFoundResponse(w, r, errors.New("Recipient not found"))
				return
			}

			httpx.InternalServerErrorResponse(w, r, errors.New("failed to fetch recipient"))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
