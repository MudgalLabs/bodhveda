package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
)

func parseContactID(r *http.Request) (int64, error) {
	return strconv.ParseInt(httpx.ParamStr(r, "contact_id"), 10, 64)
}

// --- Developer API (API-key auth; project from the key) ---

func CreateRecipientContact(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		var payload dto.CreateRecipientContactPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = apiKey.ProjectID
		payload.RecipientExtID = recipientExtID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Contact created", result)
	}
}

func ListRecipientContacts(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		result, errKind, err := s.List(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UpdateRecipientContact(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		contactID, err := parseContactID(r)
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid contact ID"))
			return
		}

		var payload dto.UpdateRecipientContactPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = apiKey.ProjectID
		payload.RecipientExtID = recipientExtID
		payload.ContactID = contactID

		result, errKind, err := s.Update(ctx, &payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Contact updated", result)
	}
}

func DeleteRecipientContact(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		contactID, err := parseContactID(r)
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid contact ID"))
			return
		}

		errKind, err := s.Delete(ctx, apiKey.ProjectID, recipientExtID, contactID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Contact deleted", nil)
	}
}

// --- Console API (session auth; project from the URL) ---

func CreateRecipientContactConsole(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		var payload dto.CreateRecipientContactPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID
		payload.RecipientExtID = recipientExtID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Contact created", result)
	}
}

func ListRecipientContactsConsole(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		result, errKind, err := s.List(ctx, projectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UpdateRecipientContactConsole(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		contactID, err := parseContactID(r)
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid contact ID"))
			return
		}

		var payload dto.UpdateRecipientContactPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID
		payload.RecipientExtID = recipientExtID
		payload.ContactID = contactID

		result, errKind, err := s.Update(ctx, &payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Contact updated", result)
	}
}

func DeleteRecipientContactConsole(s *service.RecipientContactService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		recipientExtID := strings.ToLower(httpx.ParamStr(r, "recipient_external_id"))
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_external_id required"))
			return
		}

		contactID, err := parseContactID(r)
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid contact ID"))
			return
		}

		errKind, err := s.Delete(ctx, projectID, recipientExtID, contactID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Contact deleted", nil)
	}
}
