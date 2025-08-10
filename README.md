<p align="center">
  <img src="./.github/screenshots/banner.png" alt="Bodhveda banner" />
</p>

<p align="center"><strong>Backend for your in-app notifications. You send. We deliver.</strong></p>

# Bodhveda

[Bodhveda](https://bodhveda.com) is the open source notification backend that helps you add rich, scalable in-app notifications in minutes. Whether you are shipping your first product or scaling to millions, Bodhveda handles preferences, analytics, and delivery so you can focus on what matters most.

## 🌟 Why Bodhveda?

-   **Plug-and-play:** Add notifications to your app with a simple [REST API](docs/api-reference.md) or SDK.
-   **Recipient-first:** Built-in support for recipient preferences to let them opt in/out of notifications.
-   **Built for scale:** Broadcast to hundreds of thousands of recipients in seconds.
-   **Observable:** Track delivery, seen, and clicked events out of the box.
-   **Self-hosted or Managed:** Run it yourself or use [Bodhveda Cloud](https://bodhveda.com/).

## 🧠 Who is Bodhveda for?

Bodhveda is for indie devs, product teams, and anyone who needs robust in-app notifications—**without reinventing the wheel**.

-   **Building a dev.to-style blog?** Get mentions, comments, and alerts.
-   **Running a SaaS dashboard?** Send usage, billing, or system notifications.
-   **Launching a large platform?** Scale to millions, with analytics and preferences.

**Bodhveda is your notification backend.**

## 🚀 How does it work?

### 1. **Send a Direct Notification**

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "recipient_id": "recipient_123", "channel": "posts", "topic": "post_id_123", "event": "new_comment" },
    "payload": { "title": "John commented on your post", "post_url": "url_to_post" }
  }'
```

-   **Direct notifications** are delivered instantly to a recipient.
-   **Channel, topic, and event** let you organize and respect recipient preferences.

### 2. **Fetch the Recipient’s Inbox**

```bash
curl https://api.bodhveda.com/v1/recipients/recipient_123/notifications \
  -H "Authorization: Bearer YOUR_API_KEY"
```

<details><summary>Example Response</summary>

```json
[
    {
        "id": 42069,
        "recipient_id": "recipient_123",
        "payload": {
            "title": "John commented on your post",
            "post_url": "url_to_post"
        },
        "broadcast_id": null,
        "channel": "posts",
        "topic": "post_id_123",
        "event": "new_comment",
        "seen": false,
        "clicked": false,
        "created_at": "2025-08-09T13:51:38.671616+05:30",
        "updated_at": "2025-08-09T13:51:38.671616+05:30"
    }
]
```

</details>

### 3. **Send a Broadcast Notification**

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "channel": "announcements", "topic": "product", "event": "new_feature" },
    "payload": { "title": "Bodhveda v2 is live!", "message": "Check out what is new in our latest release." }
  }'
```

-   **Broadcasts** reach all recipients subscribed to a [target](docs/core-concepts.md#notification-targeting).
-   **Preferences** are respected. No more spamming recipients who opted out.

## 🧩 Features at a Glance

-   **Direct & Broadcast Notifications**
    Send 1:1 or broadcast to hundreds of thousands of recipients in seconds, with full preference awareness.

-   **Channel / Topic / Event Targeting**
    Organize notifications for preferences, targeting and analytics.

-   **Recipient Preferences**
    Let recipients opt in/out of notification.

-   **Inbox-like API**
    Fetch, mark as seen/unseen, delete, just like a modern inbox.

-   **Analytics & Observability**
    See who received, saw, and clicked every notification.

<!-- -   **Logs Explorer**
    Inspect delivery attempts, failures, and system logs. -->

-   **Bodhveda Console**
    Your all-in-one dashboard to manage recipients, preferences, and API keys; send broadcasts or direct notifications; and monitor detailed logs, analytics, and delivery stats in real time.

-   **REST-first, SDK-friendly**
    Integrate with any stack. [SDKs](docs/api-reference.md#sdk) available.

-   **Self-hostable or Managed**
    Use our cloud or run on your own infra.

## 🗯 Learn More

-   [Quickstart Guide](docs/quickstart.md) — Send your first notification in 3 minutes.
-   [Overview](docs/overview.md) — How Bodhveda fits into your stack.
-   [API Reference](docs/api-reference.md) — Full REST API docs.
-   [Core Concepts](docs/core-concepts.md) — Understand recipients, targets, preferences, and analytics.
-   [Console](https://console.bodhveda.com) — Managed cloud dashboard.
-   [Self-hosting Guide](docs/self-host.md) (coming soon)

## 📜 License

[AGPL v3](LICENSE) — Notifications should be free to own, run, and customize.

<p align="center">
  Built with 💙 by <a href="https://mudgallabs.com" target="_blank">Mudgal Labs</a>
</p>
