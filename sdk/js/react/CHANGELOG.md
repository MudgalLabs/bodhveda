# Changelog

## 0.7.0

-   Bumped in lockstep with core `@bodhveda/js@0.7.0`, which adds **strict
    targets** (a per-project setting, off by default, that makes the catalog a
    send-time gate) and **`mandatory`** catalog entries. This package re-exports
    the updated types; no hook signature changed.
-   **`usePreferences()` and `useCheckPreference()` now return
    `state.mandatory`.** This is the one thing that matters here: a mandatory
    entry is one the recipient may not opt out of, and `useUpdatePreference` on it
    is refused with a `400`. **Render it as locked, not as a switch** — a toggle
    that saves and changes nothing is worse than one that refuses. The README's
    new "Building a settings screen" section has a worked example.
-   Strict targets itself never changes what these hooks do — they read and write
    preferences, they do not send — but it is why a target missing from
    `usePreferences()` is worth chasing: an uncataloged target appears on no
    settings screen, so no recipient can mute it. Seeding the catalog is a
    server-side `@bodhveda/js` concern (`preferences.upsertMany`), not a hook.
-   Depends on `@bodhveda/js@^0.7.0`. No API change in this package itself.

## 0.6.0

-   Bumped in lockstep with core `@bodhveda/js@0.6.0`, which makes `payload`
    optional on a send so an email can be delivered **without** creating an in-app
    notification. This package re-exports the updated types; no hook signature
    changed.
-   Worth knowing for inbox UIs: email-only notifications never appear in the
    recipient feed or unread count, so nothing here needs to filter them out. If
    your own code `switch`es exhaustively on `Notification["status"]`, add a case
    for the new `not_requested`.

## 0.5.0

-   Bumped in lockstep with core `@bodhveda/js@0.5.0`, which makes direct sends
    fully asynchronous and adds `bodhveda.notifications.get(id)` to read a
    notification's resolved in-app status and email delivery outcome. This package
    re-exports the updated `Notification` type (now with `status` / `email`), but
    `notifications.get` is a **server-side** concern (full-access key) — so there
    are **no new hooks** here, and nothing should call it from a browser.
-   Depends on `@bodhveda/js@^0.5.0`. No API change in this package itself.

## 0.4.0

-   Bumped in lockstep with core `@bodhveda/js@0.4.0`, which adds the project
    preference **catalog** client (`bodhveda.preferences`) and the idempotent
    `recipients.contacts.setPrimary`. This package re-exports the new types, but
    both are **server-side** concerns (full-access key, email addresses) — so
    there are **no new hooks** here, and nothing should call them from a browser.
-   Depends on `@bodhveda/js@^0.4.0`. No API change in this package itself.

## 0.3.0

-   **Core dependency renamed to `@bodhveda/js`** (was `bodhveda`). This package now depends on
    `@bodhveda/js@^0.3.0` and re-exports from it. If you import core types transitively through this
    package, nothing changes; if you also install core directly, switch to `@bodhveda/js`. Versions
    are re-baselined so this package, `@bodhveda/js`, and the Go SDK now share one number (`0.3.0`).
-   Bumped in lockstep with core `@bodhveda/js@0.3.0` (**preference reads now tell the truth**) —
    this package re-exports the core types, so `usePreferences()` and `useCheckPreference()` now
    return a `state` resolved by the same cascade the delivery path uses, plus the new `cataloged`
    field.
-   **This changes what a settings screen built on `usePreferences()` renders.** The hook returns
    more entries than before, and some toggle states flip to what Bodhveda actually does. See the
    core `@bodhveda/js@0.3.0` changelog for the three specific ways the old answers were wrong.
-   No new hooks, and no API change in this package itself.

## 0.1.0

-   Bumped in lockstep with core `bodhveda@0.1.0` (the email medium release, published under the
    old `bodhveda` package name) — this package
    re-exports the core types, now including the `email` send block, `deliveries[]`, recipient
    contacts, and the per-medium preference `medium`.
-   No new hooks. The React hooks remain focused on the recipient inbox (in-app notifications and
    preferences); email/contacts/provider config are server-side concerns handled via the core
    SDK.
