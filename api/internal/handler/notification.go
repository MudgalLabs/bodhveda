package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	"github.com/mudgallabs/tantra/jsonx"
	"github.com/mudgallabs/tantra/query"
)

func SendNotification(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		var payload dto.SendNotificationPayload
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.ProjectID = apiKey.ProjectID

		result, message, errKind, err := s.Send(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, message, result)
	}
}

func NotificationsOverview(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		result, errKind, err := s.Overview(ctx, projectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func ListNotifications(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var cursor query.Cursor
		err := httpx.DecodeQuery(r, &cursor)
		if err != nil {
			httpx.BadRequestResponse(w, r, err)
			return
		}

		notifications, returnedCursor, errKind, err := s.ListForRecipient(
			ctx, apiKey.ProjectID, recipientExtID, &cursor)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		result := map[string]interface{}{
			"notifications": notifications,
			"cursor":        returnedCursor,
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func UnreadCountForRecipient(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		count, errKind, err := s.UnreadCountForRecipient(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"unread_count": count})
	}
}

func MarkNotificationsAsRead(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var req dto.NotificationIDsPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		updated, errKind, err := s.MarkAsReadForRecipient(ctx, apiKey.ProjectID, recipientExtID, req.NotificationIDs)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}
		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func MarkNotificationsAsUnread(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var req dto.NotificationIDsPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		updated, errKind, err := s.MarkAsUnreadForRecipient(ctx, apiKey.ProjectID, recipientExtID, req.NotificationIDs)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func MarkAllNotificationsAsRead(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		updated, errKind, err := s.MarkAllAsReadForRecipient(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func MarkNotificationsAsOpened(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var req dto.NotificationIDsPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		updated, errKind, err := s.MarkAsOpenedForRecipient(ctx, apiKey.ProjectID, recipientExtID, req.NotificationIDs)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func MarkAllNotificationsAsOpened(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		updated, errKind, err := s.MarkAllAsOpenedForRecipient(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func DeleteNotifications(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		var req dto.NotificationIDsPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		updated, errKind, err := s.DeleteForRecipient(ctx, apiKey.ProjectID, recipientExtID, req.NotificationIDs)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}
		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}

func DeleteAllNotifications(s *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		apiKey := middleware.GetAPIKeyFromContext(ctx)

		recipientExtID := httpx.ParamStr(r, "recipient_external_id")
		if recipientExtID == "" {
			httpx.BadRequestResponse(w, r, errors.New("recipient_id required"))
			return
		}

		updated, errKind, err := s.DeleteAllForRecipient(ctx, apiKey.ProjectID, recipientExtID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}
		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]int{"updated": updated})
	}
}
