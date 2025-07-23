package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

func directHandler(app *appType) http.HandlerFunc {
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

func broadcastHandler(app *appType) http.HandlerFunc {
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

func inboxHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)
		recipient := chi.URLParam(r, "recipient")

		// Parse limit and offset to integers and apply defaults if necessary.
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

		fmt.Println("Fetching inbox for recipient:", recipient, "with limit:", limit, "and offset:", offset)

		inbox, errKind, err := app.service.NotificationService.Inbox(ctx, projectID, recipient, limit, offset)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "", inbox)
	}
}
