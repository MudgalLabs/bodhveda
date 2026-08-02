# Changelog

## 0.7.0

**Strict targets** — a new per-project setting, **off by default**. Nothing
changes for you unless you turn it on.

With it ON, Bodhveda rejects a send whose `(target, medium)` has no entry in the
project's preference catalog: `400`, nothing written, for direct and broadcast
alike. It is a maturity setting for a project whose catalog is already stable —
turn it on and a typo'd event name becomes a bug you catch on the first call
rather than the week someone asks why they were never notified.

With it OFF (the default) an uncataloged target sends as before. Note that this
was never a free pass for every medium: anything other than `in_app` still
resolves to not-delivered without a catalog entry, because that is the medium
default.

The check applies **per medium**, to the mediums a send actually asks for — a
send carrying only `payload` needs an `in_app` entry, one carrying `email` needs
an `email` entry, one carrying both needs both. A send with no `target` at all is
never gated. A `topic: "any"` catalog entry satisfies the gate for every concrete
topic beneath it, so one entry still covers an unbounded set of per-resource
targets.

-   **`mandatory` added to `CreateProjectPreferenceRequest`,
    `UpsertProjectPreferenceItem`, `UpdateProjectPreferenceRequest` and
    `ProjectPreference`.** A mandatory entry is cataloged — so sends pass the gate
    — but recipients cannot opt out of it: writing a recipient preference for it
    is refused with a `400` and any rule they already had is ignored. It is what
    makes transactional notifications (password resets, security alerts, a
    one-shot welcome) possible now that the catalog is also the preference
    surface.

    `default_enabled` still applies, so setting it `false` stops the notification:
    mandatory removes the RECIPIENT's choice, not yours.

-   **`ResolvedPreferenceState.mandatory`** is returned by
    `recipients.preferences.list()` and `.check()`. **Render mandatory cells as
    locked rather than as a switch** — a toggle that saves and changes nothing is
    worse than one that refuses.

Backwards compatible: `mandatory` is optional on every request type and defaults
to `false`, and strict targets is off unless you turn it on.

## 0.6.0

Email-only sends: deliver an email without creating an in-app notification.

-   **`payload` is now optional on `SendNotificationRequest`.** Omit it, together
    with an `email` block, and Bodhveda sends the email **without** writing
    anything to the recipient's inbox:

    ```ts
    await bodhveda.notifications.send({
        recipient_id: "recipient_123",
        target: { channel: "conversation", topic: "thread_7", event: "reply" },
        email: { subject: "3 new messages", html: "<p>…</p>" },
    });
    ```

    This exists because the two mediums often want different timing. A support
    inbox wants one in-app row per reply the instant it happens, but a single
    *debounced* email covering the last few minutes — two sends, and previously
    the second one always dropped a duplicate row into the feed.

-   **A send must carry at least one content block.** Omitting both `payload` and
    `email` is now a `400` rather than a server error. This is the guard that
    keeps an accidental omission from silently becoming a no-op.

-   **`Notification.status` gained `not_requested`** — the send carried no
    `payload`, so no in-app delivery was requested. Unlike every other status it
    is set when the send is accepted and never changes. Such notifications are
    excluded from the recipient's feed, unread count and mark-all-read, but still
    carry the `email` delivery outcome. **If you `switch` exhaustively on
    `status`, add a case for it.**

-   **`Notification.payload` is `null`** for an email-only send. The field was
    already typed `unknown`, so this is not a type change.

Backwards compatible: `payload` only became *optional*, so existing calls behave
identically. `payload` remains **required on a broadcast**, which is in-app only.

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
