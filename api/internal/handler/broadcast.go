package handler

import (
	"errors"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
)

func ListBroadcasts(s *service.BroadcastService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		payload := dto.ListBroadcastsFilters{}
		if err := httpx.DecodeQuery(r, &payload); err != nil {
			httpx.BadRequestResponse(w, r, err)
			return
		}

		payload.ProjectID = projectID

		result, errKind, err := s.List(ctx, &payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

// GetBroadcastDeliveryTree serves the console's per-medium delivery breakdown
// for one broadcast.
func GetBroadcastDeliveryTree(s *service.BroadcastService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		broadcastID, err := httpx.ParamInt(r, "broadcast_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid broadcast ID"))
			return
		}

		result, errKind, err := s.GetDeliveryTree(ctx, projectID, broadcastID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}
