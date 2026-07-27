# Changelog

## v0.6.1

Feature parity with `@bodhveda/js` / `@bodhveda/react` `0.6.0`. Email-only sends:
deliver an email without creating an in-app notification.

> **Skip `v0.6.0` — it shipped by mistake and is byte-identical to `v0.5.0`.**
> The tag was pushed before the changes below were committed, so it carries none
> of them. It is harmless (nothing broke; it is simply `v0.5.0` under another
> number) and it was deliberately NOT deleted or re-pointed: `proxy.golang.org`
> had already cached it, and re-tagging a cached version produces a
> `checksum mismatch` for every consumer that had resolved it — an error that
> reads as a supply-chain compromise. Superseding it with `v0.6.1` is the safe
> remedy.
>
> If you are already on `v0.6.0`, you can make email-only sends today: a nil
> `Payload` marshals to `"payload": null`, which the API treats as absent. This
> release only adds `omitempty` (so the key is omitted rather than sent as null)
> and the documentation below.

-   **`SendNotificationRequest.Payload` is now optional.** Leave it nil, together
    with an `Email` block, and Bodhveda sends the email **without** writing
    anything to the recipient's inbox:

    ```go
    _, err := client.Notifications.Send(ctx, bodhveda.SendNotificationRequest{
        RecipientID: bodhveda.String("recipient_123"),
        Target:      &bodhveda.Target{Channel: "conversation", Topic: "thread_7", Event: "reply"},
        Email:       &bodhveda.EmailContent{Subject: "3 new messages", HTML: "<p>…</p>"},
    })
    ```

    This exists because the two mediums often want different timing. A support
    inbox wants one in-app row per reply the instant it happens, but a single
    *debounced* email covering the last few minutes — two sends, and previously
    the second one always dropped a duplicate row into the feed.

    The field gained `omitempty`, so a nil payload omits the key rather than
    sending `"payload": null`. Both are read as absent by the API; omitting is
    just clearer on the wire. **No signature change** — `json.RawMessage` was
    already nil-able, so existing code compiles and behaves identically.

-   **A send must carry at least one content block.** Omitting both `Payload` and
    `Email` is now a `400` rather than a server error.

-   **`Notification.Status` gained `"not_requested"`** — the send carried no
    `Payload`, so no in-app delivery was requested. Unlike every other status it
    is set when the send is accepted and never changes. Such notifications are
    excluded from the recipient's feed, unread count and mark-all-read, but still
    carry the `Email` delivery outcome. **If you `switch` exhaustively on
    `Status`, add a case for it.**

-   **`Notification.Payload` is nil** for an email-only send.

`Payload` remains **required on a broadcast**, which is in-app only.

## v0.5.0

Feature parity with `@bodhveda/js` / `@bodhveda/react` `0.5.0`. Direct sends are
now fully asynchronous, and there is a new way to read a notification back.

-   **`client.Notifications.Get(ctx, notificationID)`** — retrieve a single
    notification by the id returned from `Send`. It carries the resolved in-app
    `Status` and, when the send included an `Email` block, the email delivery
    outcome on `Notification.Email` (`Status`, `SentAt`, `DeliveredAt`, `BouncedAt`,
    …). `Send` accepts the notification and returns its id; `Get` tells you what
    happened to it.
-   **`Notification` gains `Status`, `CompletedAt`, and `Email`.** `Status` is the
    in-app outcome (`enqueued` → `delivered` / `muted` / `quota_exceeded` /
    `failed`); `Email` is the per-medium email outcome (type
    `NotificationEmailDelivery`). Additive — existing fields are unchanged.
-   **`SendNotificationResponse.Deliveries` is deprecated and no longer
    populated.** A direct send now returns as soon as the notification is accepted
    (`Status: "enqueued"`); preference gating, billing, and the entire email
    fan-out run in the worker. Read the outcome back with `Notifications.Get`
    instead. The `Notification` (with its id) is still returned on send.

This release lands as `v0.5.0`, back in lockstep with JS/React — the `0.4.1` patch
lead existed only to route around the immutable `v0.4.0` placeholder tag.

## v0.4.1

Feature parity with `@bodhveda/js` / `@bodhveda/react` `0.4.0` — the new 0.4.0
Dev-API surface is now available in the Go SDK. Additive only; existing methods
are unchanged.

> These additions ship as `0.4.1`, one patch ahead of JS/React's `0.4.0`. The
> `0.4.0` Go tag (below) was published as a no-op placeholder and was already
> cached immutably by the Go module proxy, so it could not be re-pointed at this
> code — hence the bump.

-   **`client.Preferences`** — a top-level, project-scoped client for the
    preference CATALOG (the project-level entries that declare which `(target,
    medium)` pairs a project may send and the default a recipient inherits):
    `List` / `Get` / `Create` (strict — 409 on a duplicate natural key) / `Update`
    / `Delete`, plus `UpsertMany` for a declarative one-call catalog setup
    (`&UpsertProjectPreferencesOptions{Prune: true}` to also delete entries absent
    from the slice). Distinct from `client.Recipients.Preferences`, which stays a
    single recipient's own toggles. Requires a full-scope API key.
-   **`client.Recipients.Contacts.SetPrimary`** — idempotent "ensure this is the
    primary contact for this medium" (PUT). A server-side sync can keep a
    recipient's primary email current in one call instead of list → diff →
    create/update; unlike `Create`, it does not 409 when the contact already
    exists.

New types: `ProjectPreference`, `CreateProjectPreferenceRequest`,
`UpdateProjectPreferenceRequest`, `UpsertProjectPreferenceItem`,
`UpsertProjectPreferencesOptions`, `SetPrimaryContactRequest`,
`SetPrimaryContactResponse`.

## v0.4.0

Lockstep placeholder — **no changes to the Go SDK.** Tagged only to keep all
three SDKs on one version number; the 0.4.0 Dev-API additions land in `v0.4.1`
above. (Superseded — install `v0.4.1` or later for the catalog + `SetPrimary`
features.)

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
