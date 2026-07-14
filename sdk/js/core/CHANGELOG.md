# Changelog

## 0.1.0

The **email medium** release.

-   `notifications.send()` accepts an optional typed `email` block (`{ subject, html, text }`).
    Its presence makes email eligible (direct sends only). Bodhveda does no templating — you
    render the content and pass it.
-   `SendNotificationResponse` now carries `deliveries[]` — per-medium delivery outcomes for a
    direct send (email in v1).
-   New `recipients.contacts.*` API (`create`, `list`, `update`, `delete`) for per-medium
    recipient contact addresses. Email needs a primary email contact.
-   `recipients.preferences.set()` / `check()` accept an optional `medium` (`"in_app"` or
    `"email"`); in-app and email are toggled independently for the same target. Defaults to
    `"in_app"`, so existing calls are unchanged.
