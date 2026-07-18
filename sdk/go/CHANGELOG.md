# Changelog

## v0.4.0

Lockstep release with `@bodhveda/js` / `@bodhveda/react` `0.4.0`. **No changes to
the Go SDK** — the 0.4.0 Dev-API additions (project preference catalog CRUD +
bulk upsert, idempotent primary contact) are not yet surfaced in this SDK. The
tag exists only to keep all three SDKs on one version number.

## v0.3.0

**Preference reads now tell the truth.** `Recipients.Preferences.List` and `.Check` returned a
state that could contradict what Bodhveda actually delivered. They now resolve with the same
cascade the delivery path uses, so `State.Enabled` is what a send would really do.

If you render a settings screen from these, **expect values and rows to change** — the old answers
were wrong in these ways:

-   `topic: "any"` rules were ignored. A recipient rule on `posts/any/new_comment` did not affect
    the reported state of `posts/post_123/new_comment`, though it did affect delivery.
-   The default was assumed to be the same for every medium. It is not: `in_app` delivers unless
    muted, every other medium stays off unless enabled. `Check` reported `Enabled: true` for an
    email target that would never fire.
-   Recipient rules on **uncataloged** targets were invisible to `List` — while still delivering. A
    recipient could be shown "off" for an email they were actively receiving.

Breaking (source-level):

-   **`PreferenceState.Inherit` is now `PreferenceState.Inherited`.** The old field was tagged
    `json:"inherit"` while the API sends `inherited`, so it never deserialized — it was always
    `false`. Code reading `.Inherit` must be updated; it was reading a value that was never
    populated.
-   `Preference.State` and `CheckPreferenceResponse.State` are now `ResolvedPreferenceState`, which
    adds **`Cataloged`** — whether a project-level rule exists for that exact `(target, medium)`.
    Use it to decide what to render; it does **not** predict delivery. `Enabled` is the answer.
-   `SetPreferenceResponse.State` stays `PreferenceState`: it describes the rule you just wrote,
    not a resolution.

Also: `List` returns **more entries than before** — every target in your catalog plus any target
the recipient has a rule of their own for, across `in_app` and `email`. Entries can appear for
`(target, medium)` pairs you never cataloged, because they resolve and can deliver.

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
