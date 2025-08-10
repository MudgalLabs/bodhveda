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

func CreateProject(s *service.ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)

		var payload dto.CreateProjectPaylaod
		if err := jsonx.DecodeJSONRequest(&payload, r); err != nil {
			httpx.MalformedJSONResponse(w, r, err)
			return
		}

		payload.UserID = userID

		result, errKind, err := s.Create(ctx, payload)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusCreated, "Project created", result)
	}
}

func ListProjects(s *service.ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)

		result, errKind, err := s.List(ctx, userID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", result)
	}
}

func DeleteProject(s *service.ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)
		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		errKind, err := s.Delete(ctx, userID, projectID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "Project deleted. All related data will be deleted too.", nil)
	}
}
