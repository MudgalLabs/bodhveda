package handler

import (
	"errors"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
)

// --- Console API (session auth; project from the URL) ---

func GetProjectEmailSettings(s *service.ProjectEmailSettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		result, errKind, err := s.Get(ctx, projectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		// result is nil when the project has no email settings configured yet.
		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UpsertProjectEmailSettings(s *service.ProjectEmailSettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		var payload dto.UpsertProjectEmailSettingsPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID

		result, errKind, err := s.Upsert(ctx, &payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Email settings saved", result)
	}
}
