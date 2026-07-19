# Changelog

## 0.5.0

Direct sends are now fully asynchronous, and there is a new way to read a
notification back.

-   **New `bodhveda.notifications.get(notificationId)`** — retrieve a single
    notification by the id returned from `send()`. It carries the resolved in-app
    `status` and, when the send included an `email` block, the email delivery
    outcome on `notification.email` (`status`, `sent_at`, `delivered_at`,
    `bounced_at`, …). This mirrors the send/lookup pattern of transactional email
    APIs: `send()` accepts the notification and returns its id; `get()` tells you
    what happened to it.
-   **`Notification` gains `status`, `completed_at?`, and `email?`.** `status` is
    the in-app outcome (`enqueued` → `delivered` / `muted` / `quota_exceeded` /
    `failed`); `email` is the per-medium email outcome described above. Additive —
    existing fields are unchanged.
-   **`SendNotificationResponse.deliveries` is deprecated and no longer
    populated.** A direct send now returns as soon as the notification is accepted
    (`status: "enqueued"`); preference gating, billing, and the entire email
    fan-out run in the worker. Read the outcome back with `notifications.get()`
    instead. The `notification` (with its id) is still returned on send.

## 0.4.0

Additive — no breaking changes to existing methods.

-   **New `bodhveda.preferences` client** for the project preference **catalog**
    (project-scoped by the API key): `list()`, `get(id)`, `create(req)` (strict —
    409 on conflict), `update(id, req)`, `delete(id)`, and `upsertMany(prefs, {
    prune? })` for declaratively setting a whole catalog in one call. This is
    distinct from `recipients.preferences`, which stays a single recipient's own
    toggles.
-   **New `bodhveda.recipients.contacts.setPrimary(recipientId, { medium, address
    })`** — idempotently ensure an address is the primary contact for a medium
    (create-or-update, `200` either way). A server-side sync can keep a primary
    email current in one call instead of list → diff → create/update. `create`
    stays strict (409 on conflict).

Both are **server-side** concerns (they need a full-access key and touch email
addresses) — `@bodhveda/react` gains no browser-side hooks for them.

## 0.3.0

**This package is now `@bodhveda/js`.** The npm package was renamed from `bodhveda` to
`@bodhveda/js`, matching `@bodhveda/react` under one scope. The old `bodhveda` package is deprecated
and frozen at `0.1.0` — it will receive no further releases. Migrate with:

```bash
npm uninstall bodhveda && npm install @bodhveda/js
```

and change your imports:

```diff
- import { Bodhveda } from "bodhveda";
+ import { Bodhveda } from "@bodhveda/js";
```

The API is otherwise identical — only the package name changed. Versions are re-baselined so the JS
core, `@bodhveda/react`, and the Go SDK now share one number: this is `0.3.0` across all three (the
`bodhveda@0.2.0` prepared for the preference-read fix was never published; that fix ships here).

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
