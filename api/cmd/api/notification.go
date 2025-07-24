package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

func sendNotificationHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		type request struct {
			Payload json.RawMessage `json:"payload"`
		}

		var req request
		if err := decodeJSONRequest(&req, r); err != nil {
			malformedJSONResponse(w, r, err)
			return
		}

		notification, errKind, err := app.service.NotificationService.Direct(ctx, projectID, recipient, req.Payload)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "direct notification sent", notification)
	}
}

func sendBroadcastHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)

		type request struct {
			Payload json.RawMessage `json:"payload"`
		}

		var req request
		if err := decodeJSONRequest(&req, r); err != nil {
			malformedJSONResponse(w, r, err)
			return
		}

		broadcast, errKind, err := app.service.NotificationService.Broadcast(ctx, projectID, req.Payload)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "broadcast notification sent", broadcast)
	}
}

func fetchNotificationsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}

		inbox, errKind, err := app.service.NotificationService.List(ctx, projectID, recipient, limit, offset)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "", inbox)
	}
}

func fetchUnreadCountHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		count, errKind, err := app.service.NotificationService.UnreadCount(ctx, projectID, recipient)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		resp := map[string]int{"unread_count": count}
		successResponse(w, r, http.StatusOK, "", resp)
	}
}

func markNotificationsReadHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		type request struct {
			IDs []string `json:"ids"`
		}

		var req request
		if err := decodeJSONRequest(&req, r); err != nil {
			malformedJSONResponse(w, r, err)
			return
		}

		uuids := make([]uuid.UUID, 0, len(req.IDs))
		for _, idStr := range req.IDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				badRequestResponse(w, r, errors.New("invalid notification id: "+idStr))
				return
			}
			uuids = append(uuids, id)
		}

		errKind, err := app.service.NotificationService.MarkAsRead(ctx, projectID, recipient, uuids)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "notifications marked as read", nil)
	}
}

func markAllNotificationsReadHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		errKind, err := app.service.NotificationService.MarkAllAsRead(ctx, projectID, recipient)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "all notifications marked as read", nil)
	}
}

func deleteNotificationsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		type request struct {
			IDs []string `json:"ids"`
		}

		var req request
		if err := decodeJSONRequest(&req, r); err != nil {
			malformedJSONResponse(w, r, err)
			return
		}

		uuids := make([]uuid.UUID, 0, len(req.IDs))
		for _, idStr := range req.IDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				badRequestResponse(w, r, errors.New("invalid notification id: "+idStr))
				return
			}
			uuids = append(uuids, id)
		}

		errKind, err := app.service.NotificationService.Delete(ctx, projectID, recipient, uuids)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "notifications deleted", nil)
	}
}

func deleteAllNotificationsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		errKind, err := app.service.NotificationService.DeleteAll(ctx, projectID, recipient)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "all notifications deleted", nil)
	}
}
