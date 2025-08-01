package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/mudgallabs/tantra/auth/session"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/logger"
	"go.uber.org/zap"
)

type contextKey string

const ctxUserIDKey contextKey = "user_id"
const ctxUserTimezoneKey contextKey = "user_timezone"
const ctxProjectIDKey contextKey = "project_id"

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
				httpx.UnauthorizedErrorResponse(w, r, errorMsg, errors.New("no session found"))
				return
			}
		}

		userID := session.Manager.GetInt(ctx, "user_id")

		if userID == 0 {
			l.Warnw("no user ID found in the session")
			httpx.UnauthorizedErrorResponse(w, r, errorMsg, errors.New("no user ID found in session"))
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

// GetProjectIDFromContext allows downstream access
func GetProjectIDFromContext(ctx context.Context) int {
	id, ok := ctx.Value(ctxProjectIDKey).(int)
	if !ok {
		panic("project ID not valid in context")
	}
	return id
}

func APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			httpx.UnauthorizedErrorResponse(w, r, "missing Authorization header", errors.New("missing Authorization header"))
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.UnauthorizedErrorResponse(w, r, "invalid Authorization header format", errors.New("invalid Authorization header format"))
			return
		}

		projectID := 1
		// Add projectID to context
		ctx = context.WithValue(ctx, ctxProjectIDKey, projectID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
