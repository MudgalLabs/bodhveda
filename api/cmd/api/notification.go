package main

import (
	"bodhveda/internal/feature/notification"
	"net/http"
)

func directHandler(app *appType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID := getProjectIDFromContext(ctx)

		var payload notification.DirectPayload
		if err := decodeJSONRequest(&payload, r); err != nil {
			malformedJSONResponse(w, r, err)
			return
		}

		notification, errKind, err := app.service.NotificationService.Direct(ctx, projectID, &payload)
		if err != nil {
			serviceErrResponse(w, r, errKind, err)
			return
		}

		successResponse(w, r, http.StatusOK, "direct notification sent", notification)
	}
}
