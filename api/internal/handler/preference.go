package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
)

func CreateProjectPreference(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		var payload dto.CreateProjectPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID

		result, errKind, err := s.CreateProjectPreference(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Project preference created", result)
	}
}

func ListProjectPreferences(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		kind := httpx.QueryStr(r, "kind")

		if kind == "" {
			// Default to project if not specified
			kind = "project"
		}

		if kind != "project" && kind != "recipient" {
			httpx.BadRequestResponse(w, r, errors.New("Invalid preference kind"))
			return
		}

		fmt.Printf("Listing preferences for project %d with kind %s\n", projectID, kind)

		switch kind {
		case "project":
			result, errKind, err := s.ListProjectPreferences(ctx, projectID)
			if err != nil {
				httpx.ServiceErrResponse(w, r, errKind, err)
				return
			}

			httpx.SuccessResponse(w, r, http.StatusOK, "", result)
			return
		case "recipient":
			result, errKind, err := s.ListRecipientPreferences(ctx, projectID)
			if err != nil {
				httpx.ServiceErrResponse(w, r, errKind, err)
				return
			}

			httpx.SuccessResponse(w, r, http.StatusOK, "", result)
			return
		}

		httpx.BadRequestResponse(w, r, errors.New("Invalid preference kind"))
	}
}

func UpsertRecipientPreferences(s *service.PreferenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := httpx.ParamStr(r, "recipient_id")

		var payload dto.UpsertRecipientPreferencePayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID
		payload.RecipientExtID = recipientExtID

		result, errKind, err := s.UpsertRecipientPreference(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Recipient preference updated", result)
	}
}
