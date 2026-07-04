# TODO

## 🔴 Security — recipient sub-routes are not recipient-scoped (cross-recipient IDOR)

**Status:** open — needs an auth-model decision.
**Found:** while auditing Grahak's notification center (a Bodhveda customer). Applies to **any** customer using a client-side (non–full-scope) API key.

### What's wrong

The recipient sub-routes in `api/cmd/api/routes.go` (the Developer API group) —

```
/recipients/{recipient_external_id}/notifications      GET   (list)
/recipients/{recipient_external_id}/notifications/unread-count  GET
/recipients/{recipient_external_id}/notifications      PATCH (mark read/opened)
/recipients/{recipient_external_id}/notifications      DELETE
/recipients/{recipient_external_id}/preferences        GET / PATCH / GET check
```

are gated **only** by `APIKeyBasedAuthMiddleware`. They do **not** carry
`VerifyAPIKeyHasFullScope`, and nothing binds the caller to a recipient:

- `APIKeyBasedAuthMiddleware` (`api/internal/middleware/middleware.go`) resolves the
  bearer key to a **project** (`apiKey.ProjectID`) — never to a recipient.
- `recipient_external_id` is taken **straight from the URL path**.
- `CreateRecipientIfNotExists` even auto-provisions whatever id is named.
- CORS on the group is `AllowedOrigins: "*"`, so it's callable from any origin.

So the `{recipient_external_id}` in the path is entirely attacker-controlled, and a
**public/client key is sufficient** to reach these endpoints (only the recipient
*entity* routes — `GET/PATCH/DELETE /recipients/{id}` — and `POST /notifications/send`
require full scope).

### Exploit

A client app ships its client key to the browser (that's the intended model). Any
visitor can extract it, then for any other recipient id `V` in the same project:

- `GET    /recipients/V/notifications` → **read V's entire feed** (every notification
  payload — for Grahak that's customer-message previews, @mention context, workspace
  invite tokens, etc.)
- `DELETE /recipients/V/notifications` → **delete V's notifications**
- `PATCH  /recipients/V/notifications` → mark V's notifications read/opened
- `GET/PATCH /recipients/V/preferences` → read / flip V's preferences

All of a customer's end users live in **one** Bodhveda project, so project-scoping
gives **zero** isolation between recipients. Recipient ids aren't sequential, but
they aren't secret either — in Grahak, a notification's own `actor.id` (delivered to
the browser) is a *teammate's* recipient id, so any authenticated user can pivot to a
colleague's feed.

**Impact:** confidentiality (read any recipient's notification content) **and**
integrity/availability (delete/alter any recipient's notifications & preferences).
It does *not* grant access to the customer's underlying data (conversations,
messages) — only whatever is denormalized into the notification payloads — but for
Grahak the payloads themselves carry message previews and invite tokens.

### Fix direction

The authorization boundary must be **per-recipient**, not per-project. A bare public
project key must not authorize reads/writes for an arbitrary `recipient_external_id`.
Options (roughly increasing effort):

1. **Recipient-bound token (preferred).** Have the server key mint a short-lived,
   signed token scoped to a single recipient; the client uses *that* on the recipient
   sub-routes, and the handler derives the recipient from the token (ignoring — or
   requiring a match with — the path param). This is exactly how Grahak isolates
   widget end users today (single-use identity-JWT exchange), and the notification
   center should use the same shape instead of a raw project key + client-supplied id.
2. **Recipient claim on the client key.** If keys stay long-lived, a "client"-scope
   key must be bound to a recipient (issued per recipient) so it can only touch its
   own sub-routes.
3. **At minimum:** add a middleware on the `/{recipient_external_id}/notifications`
   and `/preferences` groups that requires the credential to *prove* it is that
   recipient — reject a plain project key acting on someone else's id.

Keep `send` full-scope-only (already correct). Also reconsider whether
`CreateRecipientIfNotExists` should run on read-only routes with a client key (it
lets an unauthenticated caller mint recipients).

### Cross-reference

Grahak side is fine given this constraint — it only ever passes the signed-in user's
own id — but it can't paper over the gap because the public key + client-supplied id
*is* the exposure. See Grahak's `apps/console/app/(console)/notifications-provider.tsx`
(where the client key + `recipientID` are handed to the browser) and its
`docs/team-collaboration.md`. This mirrors the widget's token-isolation model
(`widget.grahak.dev`, per-user JWT handoff) — apply the same principle here.
