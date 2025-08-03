package handler

import (
	"errors"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
)

func CreateAPIKey(s *service.APIKeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		var payload dto.CreateAPIKeyPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.UserID = userID
		payload.ProjectID = projectID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "API Key created", result)
	}
}

func ListAPIKeys(s *service.APIKeyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		result, errKind, err := s.List(ctx, userID, projectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}
