package main

import (
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/feature/user_profile"
	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/tantra/httpx"
)

func getMeHandler(s *user_profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)

		userProfile, errKind, err := s.GetUserMe(ctx, userID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", userProfile)
	}
}
