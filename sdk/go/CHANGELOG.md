# Changelog

## v0.2.0

The **email medium** release.

-   `SendNotificationRequest` accepts an optional typed `Email` block (`EmailContent{ Subject,
    HTML, Text }`). Its presence makes email eligible (direct sends only). Bodhveda does no
    templating — you render the content and pass it.
-   `SendNotificationResponse` now carries `Deliveries []*NotificationDelivery` — per-medium
    delivery outcomes for a direct send (email in v1).
-   New `client.Recipients.Contacts.*` API (`Create`, `List`, `Update`, `Delete`) for per-medium
    recipient contact addresses. Email needs a primary email contact.
-   `SetPreferenceRequest` / `CheckPreferenceRequest` gained an optional `Medium`
    (`MediumInApp` / `MediumEmail`); in-app and email are toggled independently for the same
    target. Defaults to `in_app`, so existing calls are unchanged.
