package handler

import (
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/middleware"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
)

func GetUserMeBilling(s *service.BillingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserIDFromContext(ctx)

		subscription, errKind, err := s.GetSubscription(ctx, userID)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		usage, errKind, err := s.GetUsage(ctx, userID, subscription.PlanID, subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd)
		if err != nil {
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", map[string]any{
			"subscription": subscription,
			"usage":        usage,
		})
	}
}
