# Changelog

## 0.2.0

**Preference reads now tell the truth.** `recipients.preferences.list()` and `.check()` returned a
state that could contradict what Bodhveda actually delivered. They now resolve with the same
cascade the delivery path uses, so `state.enabled` is what a send would really do.

If you render a settings screen from these, **expect values and rows to change** — the old answers
were wrong in these ways:

-   `topic: "any"` rules were ignored. A recipient rule on `posts/any/new_comment` did not affect
    the reported state of `posts/post_123/new_comment`, though it did affect delivery.
-   The default was assumed to be the same for every medium. It is not: `in_app` delivers unless
    muted, every other medium stays off unless enabled. `check()` reported `enabled: true` for an
    email target that would never fire.
-   Recipient rules on **uncataloged** targets were invisible to `list()` — while still delivering.
    A recipient could be shown "off" for an email they were actively receiving.

Consequences worth planning for:

-   `list()` returns **more entries than before**: every target in your catalog plus any target the
    recipient has a rule of their own for, across `in_app` and `email`. Entries can appear for
    `(target, medium)` pairs you never cataloged, because they resolve and can deliver.
-   `Preference["state"]` and `CheckPreferenceResponse["state"]` are now
    `ResolvedPreferenceState`, which adds **`cataloged`** — whether a project-level rule exists for
    that exact `(target, medium)`. Use it to decide what to render; it does **not** predict
    delivery. `enabled` is the answer.
-   `SetPreferenceResponse["state"]` is unchanged (`PreferenceState`): it describes the rule you
    just wrote, not a resolution.

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
