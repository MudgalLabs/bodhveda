# Changelog

## 0.1.0

-   Bumped in lockstep with core `bodhveda@0.1.0` (the email medium release) — this package
    re-exports the core types, now including the `email` send block, `deliveries[]`, recipient
    contacts, and the per-medium preference `medium`.
-   No new hooks. The React hooks remain focused on the recipient inbox (in-app notifications and
    preferences); email/contacts/provider config are server-side concerns handled via the core
    SDK.
