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
