package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

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

		broadcast, errKind, err := app.service.BroadcastService.Send(ctx, projectID, req.Payload)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "broadcast notification sent", broadcast)
	}
}

func fetchBroadcastsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)

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

		broadcasts, total, errKind, err := app.service.BroadcastService.List(ctx, projectID, limit, offset)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		resp := map[string]any{
			"broadcasts": broadcasts,
			"total":      total,
		}

		successResponse(w, r, http.StatusOK, "", resp)
	}
}

func deleteBroadcastsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)

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

		errKind, err := app.service.BroadcastService.Delete(ctx, projectID, uuids)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "broadcasts deleted", nil)
	}
}

func deleteAllBroadcastsHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)

		count, errKind, err := app.service.BroadcastService.DeleteAll(ctx, projectID)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "all broadcasts deleted", map[string]any{
			"count": count,
		})
	}
}
