# 🧱 Core Concepts

Understand how Bodhveda models notifications, preferences, and analytics.

---

## Recipient Model <!-- recipient-model -->

-   **recipient_id:** Unique identifier (email, user ID, etc.)
-   **name:** Optional display name
-   **metadata:** (future) Custom fields for segmentation

Example:

```json
{
    "recipient_id": "user@example.com",
    "name": "Jane Doe"
}
```

---

## Notification Targeting <!-- notification-targeting -->

Notifications are routed using a **target**:  
`channel:topic:event`

-   **Channel:** Broad category (e.g., `marketing`)
-   **Topic:** Sub-context (e.g., `pricing`, `none`, or `any`)
-   **Event:** Action (e.g., `new_feature`)

**Rules:**

-   All three fields are required for preference-aware notifications.
-   `"topic": "any"` means all topics under the channel/event.
-   `"topic": "none"` means only the base channel/event.

---

## Preferences <!-- preferences -->

-   **Global (project-level):** Default preferences for all recipients.
-   **Recipient-level:** User-specific overrides.
-   **Opt-in/out:** Recipients can subscribe or mute any target.
-   **Wildcards:** `"any"` and `"none"` for flexible targeting.

---

## Direct vs Broadcast Notifications <!-- direct-vs-broadcast-notifications -->

-   **Direct:** Sent to a single recipient (creates recipient on the fly if needed).
-   **Broadcast:** Sent to all recipients matching a target and their preferences.

---

## Delivery vs Materialization <!-- delivery-vs-materialization -->

-   **Delivery:** Attempt to notify a recipient (may be skipped if unsubscribed).
-   **Materialization:** Notification is actually created and stored for the recipient.

**Example:**  
If a user is unsubscribed from `marketing:any:new_feature`, a broadcast to that target will not materialize a notification for them.

---

## Analytics

Bodhveda tracks:

-   **Delivered:** Notification was created for the recipient.
-   **Opened:** User opened the notification.
-   **Read:** User marked the notification as read.

**Caveats:**

-   Analytics are only tracked for materialized notifications.
-   If a notification is not delivered (due to preferences), it is not counted.

---

## Glossary

-   **Recipient:** User or entity that receives notifications.
-   **Target:** Combination of channel, topic, and event.
-   **Preference:** Subscription/mute rules for targets.
-   **Materialization:** Notification is stored for recipient.
-   **Delivery:** Attempt to notify (may be skipped).

---

## Further Reading

-   [Quickstart Guide](./quickstart.md)
-   [API Reference](./api-reference.md)
