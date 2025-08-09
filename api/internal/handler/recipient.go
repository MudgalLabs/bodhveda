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

func CreateRecipient(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		var payload dto.CreateRecipientPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Recipient created", result)
	}
}

func ListRecipients(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		result, errKind, err := s.List(ctx, projectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func CreateRecipientWithAPIKey(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.BadRequestResponse(w, r, errors.New("API key required"))
			return
		}

		var payload dto.CreateRecipientPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = apiKey.ProjectID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Recipient created", result)
	}
}

func GetRecipient(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.BadRequestResponse(w, r, errors.New("API key required"))
			return
		}
		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		result, errKind, err := s.Get(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func BatchCreateRecipients(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.BadRequestResponse(w, r, errors.New("API key required"))
			return
		}

		var payload dto.BatchCreateRecipientsPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		for i := range payload.Recipients {
			payload.Recipients[i].ProjectID = apiKey.ProjectID
		}

		result, errKind, err := s.BatchCreate(ctx, payload.Recipients)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "", result)
	}
}

func UpdateRecipient(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.BadRequestResponse(w, r, errors.New("API key required"))
			return
		}

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		var payload dto.UpdateRecipientPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		result, errKind, err := s.Update(ctx, apiKey.ProjectID, recipientExtID, &payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Recipient updated", result)
	}
}

func DeleteRecipient(s *service.RecipientService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)
		if apiKey == nil {
			httpx.BadRequestResponse(w, r, errors.New("API key required"))
			return
		}
		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}
		errKind, err := s.Delete(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}
		httpx.SuccessResponse(w, r, http.StatusOK, "Recipient deleted", nil)
	}
}
