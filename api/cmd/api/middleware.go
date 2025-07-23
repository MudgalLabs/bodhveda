package main

import (
	"bodhveda/internal/logger"
	"bodhveda/internal/session"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const ctxUserIDKey contextKey = "user_id"
const ctxUserTimezoneKey contextKey = "user_timezone"
const ctxProjectIDKey contextKey = "project_id"

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromCtx(ctx)
		errorMsg := "You need to be signed in to use this route. POST /auth/sign-in to sign in."

		// Check if the session cookie exists
		_, err := r.Cookie("session")
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				l.Warn("no session found")
				unauthorizedErrorResponse(w, r, errorMsg, errors.New("no session found"))
				return
			}
		}

		userID := session.Manager.GetString(ctx, "user_id")

		if userID == "" {
			l.Warnw("no user ID found in the session")
			unauthorizedErrorResponse(w, r, errorMsg, errors.New("no user ID found in session"))
			return
		}

		l.Debugw("user ID found in the session", "user_id", userID)

		// Extend the session lifetime.
		session.Manager.SetDeadline(ctx, time.Now().Add(session.Lifetime))

		ctx = context.WithValue(ctx, ctxUserIDKey, userID)

		// Add `user_id` to this ctx's logger.
		l = l.With(zap.String(string(ctxUserIDKey), userID))
		lrw := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// the logger is associated with the request context here
		// so that it may be retrieved in subsequent `http.Handlers`
		r = r.WithContext(logger.WithCtx(ctx, l))

		next.ServeHTTP(lrw, r)
	})
}

func getUserIDFromContext(ctx context.Context) uuid.UUID {
	idStr, ok := ctx.Value(ctxUserIDKey).(string)
	if !ok {
		panic("user ID not valid in context")
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		panic(fmt.Sprintf("user ID is not uuid: %s", err.Error()))
	}
	return id
}

const requestIDCtxKey = "request_id"

func attachLoggerMiddleware(next http.Handler) http.Handler {
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

func logRequestMiddleware(next http.Handler) http.Handler {
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

func timezoneMiddleware(next http.Handler) http.Handler {
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

func getUserTimezoneFromCtx(ctx context.Context) *time.Location {
	loc, ok := ctx.Value(ctxUserTimezoneKey).(*time.Location)
	if !ok || loc == nil {
		return time.UTC
	}
	return loc
}

// getProjectIDFromContext allows downstream access
func getProjectIDFromContext(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(ctxProjectIDKey).(uuid.UUID)
	if !ok {
		panic("project ID not valid in context")
	}
	return id
}

func apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromCtx(ctx)

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			unauthorizedErrorResponse(w, r, "missing Authorization header", errors.New("missing Authorization header"))
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || parts[0] != "Bearer" {
			unauthorizedErrorResponse(w, r, "invalid Authorization header format", errors.New("invalid Authorization header format"))
			return
		}

		apiKey := parts[1]
		projectID, err := app.service.ProjectService.GetProjectFromAPIKey(apiKey)
		if err != nil {
			l.Warn("invalid API key", "err", err)
			unauthorizedErrorResponse(w, r, "invalid API key", errors.New("invalid API key"))
			return
		}

		// Add projectID to context
		ctx = context.WithValue(ctx, ctxProjectIDKey, projectID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
