package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	tantraService "github.com/mudgallabs/tantra/service"
)

// maxWebhookBodySize caps the inbound webhook body. Resend delivery events are a
// few KB; 256 KB is a generous ceiling that still bounds abuse.
const maxWebhookBodySize = 256 << 10

// EmailWebhook is the PUBLIC provider webhook ingestion endpoint (Phase 5). It is
// mounted OUTSIDE the developer API-key auth/CORS/rate-limit group and the console
// session group: it is called by the email provider (Resend via Svix), not by a
// customer, and authentication IS the webhook signature (verified in the service
// against the project's stored signing secret). An invalid/absent signature → 401.
//
// The project is resolved from the URL path (`/webhooks/email/{project_id}`), so a
// project configures exactly this URL as its Resend webhook endpoint and pastes the
// signing secret Resend generates into its Bodhveda email settings.
func EmailWebhook(s *service.EmailWebhookService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		projectID, err := httpx.ParamInt(r, "project_id")
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Invalid project ID"))
			return
		}

		// The raw body must be read verbatim — the signature is computed over the
		// exact bytes, so we cannot decode-then-reencode.
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
		if err != nil {
			httpx.BadRequestResponse(w, r, errors.New("Failed to read request body"))
			return
		}

		errKind, err := s.Ingest(ctx, projectID, r.Header, body)
		if err != nil {
			if errKind == tantraService.ErrUnauthorized {
				httpx.UnauthorizedResponse(w, r, "Webhook signature verification failed", err)
				return
			}
			httpx.ServiceErrResponse(w, r, errKind, err)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "", nil)
	}
}
