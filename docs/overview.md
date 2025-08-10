# 🧭 Bodhveda Overview

## What is Bodhveda?

Bodhveda is a modern notification infrastructure for product teams. It lets you deliver in-app notifications to your users, manage their preferences, and track delivery analytics—all without building your own backend.

-   **Plug-and-play:** Integrate via REST API or SDK.
-   **User-centric:** Built-in support for user preferences and opt-outs.
-   **Analytics:** Track delivery, open, and read events.

---

## How It Works

1. **Create a Project** in the [Console](https://console.bodhveda.com).
2. **Add Recipients** (users) via API or Console.
3. **Send Notifications** using [Direct](./core-concepts.md#direct-vs-broadcast-notifications) or [Broadcast](./core-concepts.md#direct-vs-broadcast-notifications) modes.
4. **Users receive notifications** in-app, based on their [preferences](./core-concepts.md#preferences).
5. **Track analytics** for delivery, seen, and clicked events.

---

## Key Concepts

### [Recipients](./core-concepts.md#recipient-model)

A recipient is any user or entity that can receive notifications. Each recipient has a unique `recipient_id` (e.g., email, user ID).

### [Channels, Topics, Events](./core-concepts.md#notification-targeting)

Notifications are organized by:

-   **Channel:** High-level category (e.g., `marketing`, `announcements`)
-   **Topic:** Sub-category or context (e.g., `pricing`, `post_id_123`)
-   **Event:** Specific action (e.g., `new_feature`, `new_comment`)

Together, they form a **[target](./core-concepts.md#notification-targeting)**: `channel:topic:event`.

### [Preferences](./core-concepts.md#preferences)

Recipients can subscribe or mute specific notification targets. Preferences can be set globally (project-level) or per recipient.

### [Direct vs Broadcast Notifications](./core-concepts.md#direct-vs-broadcast-notifications)

-   **Direct:** Sent to a single recipient.
-   **Broadcast:** Sent to all recipients subscribed to a target.

### [Delivery vs Materialization](./core-concepts.md#delivery-vs-materialization)

-   **Delivery:** Attempt to send a notification (may be skipped if muted).
-   **Materialization:** Notification is actually created and stored for the recipient.

---

## Use Cases

-   **In-App Notifications:** Show notifications in your web or mobile app.
-   **User Preferences:** Let users opt in/out of specific notification types.
-   **Analytics:** Track which notifications are delivered, opened, or read.

_Future: Mobile/Web Push support coming soon!_

---

## Performance

Broadcast notifications are delivered quickly—even to 100,000+ recipients in just a few seconds.

---

## Next Steps

-   [Quickstart Guide](./quickstart.md)
-   [Core Concepts](./core-concepts.md)
-   [API Reference](./api-reference.md)
