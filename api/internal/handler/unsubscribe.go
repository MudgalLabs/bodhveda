package handler

import (
	"errors"
	"fmt"
	"html"
	"net/http"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	tantraService "github.com/mudgallabs/tantra/service"
)

// UnsubscribeEmail is the PUBLIC one-click email unsubscribe endpoint (Phase 6).
// Like the provider webhook, it is mounted OUTSIDE the developer API-key group,
// the console session group, and the per-IP rate limiter: it is hit from a mail
// client with no session/API key — the signed token IS the auth. It identifies
// (project, recipient, target); hitting it disables the recipient's email
// preference for that target. Both methods are idempotent.
//
//   - POST → RFC 8058 one-click (auto-POSTed by Gmail/Yahoo). Flips the pref and
//     returns 200 with no meaningful body.
//   - GET  → renders a minimal confirmation page (also flips the pref, so a human
//     clicking the link in the email is unsubscribed too).
func UnsubscribeEmail(s *service.UnsubscribeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		isGET := r.Method == http.MethodGet

		token := r.URL.Query().Get("t")
		if token == "" {
			unsubscribeError(w, r, isGET, http.StatusBadRequest, "This unsubscribe link is missing its token.")
			return
		}

		target, errKind, err := s.UnsubscribeEmail(ctx, token)
		if err != nil {
			status := http.StatusBadRequest
			msg := "This unsubscribe link is invalid."
			if errKind == tantraService.ErrUnauthorized {
				status = http.StatusUnauthorized
				msg = "This unsubscribe link has expired."
			}
			unsubscribeError(w, r, isGET, status, msg)
			return
		}

		if isGET {
			unsubscribePage(w, target)
			return
		}

		httpx.SuccessResponse(w, r, http.StatusOK, "You have been unsubscribed.", nil)
	}
}

// unsubscribeError renders an HTML error page for GET (a human in a browser) and a
// JSON error for POST (the mailbox provider's automated one-click request).
func unsubscribeError(w http.ResponseWriter, r *http.Request, isGET bool, status int, msg string) {
	if isGET {
		writeUnsubscribeHTML(w, status, "Unsubscribe failed", msg)
		return
	}
	if status == http.StatusUnauthorized {
		httpx.UnauthorizedResponse(w, r, msg, errors.New(msg))
		return
	}
	httpx.BadRequestResponse(w, r, errors.New(msg))
}

// unsubscribePage renders the confirmation page shown when a human opens the
// unsubscribe link in their browser.
func unsubscribePage(w http.ResponseWriter, target dto.Target) {
	desc := html.EscapeString(fmt.Sprintf("%s / %s / %s", target.Channel, target.Topic, target.Event))
	body := fmt.Sprintf("You've been unsubscribed from <strong>%s</strong> emails.", desc)
	writeUnsubscribeHTML(w, http.StatusOK, "Unsubscribed", body)
}

// writeUnsubscribeHTML writes a minimal self-contained confirmation/error page.
func writeUnsubscribeHTML(w http.ResponseWriter, status int, heading, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — Bodhveda</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
         background: #0b0b0f; color: #e7e7ea; margin: 0; min-height: 100vh;
         display: flex; align-items: center; justify-content: center; }
  .card { max-width: 28rem; margin: 1.5rem; padding: 2rem; background: #16161d;
          border: 1px solid #26262f; border-radius: 12px; text-align: center; }
  h1 { font-size: 1.25rem; margin: 0 0 0.75rem; }
  p { font-size: 0.95rem; line-height: 1.5; color: #b6b6bd; margin: 0; }
  .brand { margin-top: 1.5rem; font-size: 0.8rem; color: #6b6b74; }
</style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>%s</p>
    <div class="brand">Powered by Bodhveda</div>
  </div>
</body>
</html>`, html.EscapeString(heading), html.EscapeString(heading), body)
}
