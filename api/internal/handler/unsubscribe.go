package handler

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/service"
	"github.com/mudgallabs/tantra/httpx"
	tantraService "github.com/mudgallabs/tantra/service"
)

// UnsubscribeEmail is the PUBLIC one-click email unsubscribe endpoint (Phase 6).
// Like the provider webhook, it is mounted OUTSIDE the developer API-key group,
// the console session group, and the per-IP rate limiter: it is hit from a mail
// client with no session/API key — the signed token IS the auth. It identifies
// (project, recipient, target); flipping the recipient's email preference for that
// target OFF.
//
//   - GET  → SIDE-EFFECT-FREE. Only verifies the token and renders a confirmation
//     page whose button POSTs back here. GET must not mutate: mail scanners, link
//     prefetchers, and List-Unsubscribe header fetchers issue GET requests, so a
//     mutating GET would silently unsubscribe recipients who never clicked.
//   - POST → performs the unsubscribe (idempotent). This is both the RFC 8058
//     one-click target (auto-POSTed by Gmail/Yahoo, which want a bare 200) and the
//     confirmation form's submit (which wants an HTML page — distinguished by the
//     form's `web=1` field).
func UnsubscribeEmail(s *service.UnsubscribeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		isGET := r.Method == http.MethodGet

		token := r.URL.Query().Get("t")
		if token == "" {
			if isGET {
				unsubscribeErrorPage(w, http.StatusBadRequest, "This unsubscribe link is missing its token.")
				return
			}
			httpx.BadRequestResponse(w, r, errors.New("This unsubscribe link is missing its token."))
			return
		}

		// GET: verify only, render the confirmation form. No preference is changed.
		if isGET {
			target, errKind, err := s.PreviewEmailUnsubscribe(token)
			if err != nil {
				status, msg := unsubscribeErrorFor(errKind)
				unsubscribeErrorPage(w, status, msg)
				return
			}
			unsubscribeConfirmPage(w, target, token)
			return
		}

		// POST: perform the flip. A browser form submit carries web=1 and wants HTML;
		// a provider one-click carries no such field and wants a bare 200 JSON.
		fromWeb := r.FormValue("web") == "1"

		target, errKind, err := s.UnsubscribeEmail(ctx, token)
		if err != nil {
			if fromWeb {
				status, msg := unsubscribeErrorFor(errKind)
				unsubscribeErrorPage(w, status, msg)
				return
			}
			if errKind == tantraService.ErrUnauthorized {
				httpx.UnauthorizedResponse(w, r, "This unsubscribe link has expired.", err)
				return
			}
			httpx.BadRequestResponse(w, r, err)
			return
		}

		if fromWeb {
			unsubscribeDonePage(w, target)
			return
		}
		httpx.SuccessResponse(w, r, http.StatusOK, "You have been unsubscribed.", nil)
	}
}

// unsubscribeErrorFor maps a service error kind onto the HTTP status + human
// message shown on the unsubscribe surface.
func unsubscribeErrorFor(errKind tantraService.Error) (int, string) {
	if errKind == tantraService.ErrUnauthorized {
		return http.StatusUnauthorized, "This unsubscribe link has expired."
	}
	return http.StatusBadRequest, "This unsubscribe link is invalid."
}

// unsubscribeConfirmPage renders the GET confirmation page: it describes the target
// and offers a button that POSTs back to perform the unsubscribe. The token is
// carried in the form action's query string (so the same handler reads it from `t`,
// exactly like the provider one-click POST), and `web=1` marks the submit as
// browser-originated so the POST responds with HTML.
func unsubscribeConfirmPage(w http.ResponseWriter, target dto.Target, token string) {
	desc := html.EscapeString(fmt.Sprintf("%s / %s / %s", target.Channel, target.Topic, target.Event))
	action := html.EscapeString("/unsubscribe/email?t=" + url.QueryEscape(token))
	body := fmt.Sprintf(
		`<p>Unsubscribe from <strong>%s</strong> emails?</p>`+
			`<form method="post" action="%s">`+
			`<input type="hidden" name="web" value="1">`+
			`<button type="submit" class="btn">Unsubscribe</button>`+
			`</form>`,
		desc, action,
	)
	writeUnsubscribeHTML(w, http.StatusOK, "Unsubscribe", body)
}

// unsubscribeDonePage renders the confirmation shown after a human submits the
// unsubscribe form.
func unsubscribeDonePage(w http.ResponseWriter, target dto.Target) {
	desc := html.EscapeString(fmt.Sprintf("%s / %s / %s", target.Channel, target.Topic, target.Event))
	body := fmt.Sprintf("<p>You've been unsubscribed from <strong>%s</strong> emails.</p>", desc)
	writeUnsubscribeHTML(w, http.StatusOK, "Unsubscribed", body)
}

// unsubscribeErrorPage renders the HTML error page (GET and browser-form POST).
func unsubscribeErrorPage(w http.ResponseWriter, status int, msg string) {
	writeUnsubscribeHTML(w, status, "Unsubscribe failed", "<p>"+html.EscapeString(msg)+"</p>")
}

// writeUnsubscribeHTML writes a minimal self-contained confirmation/error page.
// `body` is raw HTML the caller has already escaped where it interpolates
// untrusted values; `heading` is escaped here.
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
  p { font-size: 0.95rem; line-height: 1.5; color: #b6b6bd; margin: 0 0 1.25rem; }
  .btn { appearance: none; border: 0; cursor: pointer; font: inherit; font-weight: 600;
         padding: 0.6rem 1.4rem; border-radius: 8px; background: #e7e7ea; color: #0b0b0f; }
  .btn:hover { background: #ffffff; }
  .brand { margin-top: 1.5rem; font-size: 0.8rem; color: #6b6b74; }
</style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    %s
    <div class="brand">Powered by Bodhveda</div>
  </div>
</body>
</html>`, html.EscapeString(heading), html.EscapeString(heading), body)
}
