<p align="center">
  <img src="./.github/screenshots/banner.png" alt="Bodhveda banner" />

</p>

<p align="center"><strong>Backend for your in-app notifications. You send. We deliver.</strong></p>

# Bodhveda

[Bodhveda](https://bodhveda.com/) is an open-source Notification-as-a-Service backend for in-app notifications that helps you build rich, scalable in-app notification systems — from indie side projects to GitHub or YouTube like platforms.

## 🧠 Who is Bodhveda for?

Bodhveda is for indie devs, small teams, and product builders who need in-app notifications with preference management, analytics, and reliable, scalable delivery without reinventing the wheel, so they can focus on what matters.

### Whether you're building:

-   A **dev.to-style blog** with mentions and comments,
-   A **SaaS dashboard** that sends usage alerts,
-   Or a **large scale platform** like GitHub or YouTube,

**Bodhveda has your backend covered.**

## 🚀 How it works? It's just a [REST API](docs/API.md).

### 🎯 Send a Direct Notification

```bash
curl -X POST https://api.bodhveda.com/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "user_123",
    "payload": {
      "title": "John commented on your post",
      "post_url": "url_to_post"
    },
    "channel": "post", // Allowed to be null or "" or omitted if `recipient` provided.
    "topic": "post_123", // Same as above.
    "event": "post.new_comment" // Same as above
  }'
```

Tell us **who** to send it to (`recipient`) and **what** to send (`payload`) — Bodhveda takes care of the rest.

For direct notifications, `channel`, `topic` and `event` is needed to respect user's preferences (user muting **all** `post` notifications) and user's specific mutes (user muting **a** post `post` + `post_123`).

### 📥 Fetch the User's Inbox

```bash
curl https://api.bodhveda.com/v1/recipients/user_123/notifications
```

<details><summary>Example Response</summary>

> Notification delivered to the Recipient.

```json
[
    {
        "payload": {
            "title": "John commented on your post",
            "post_url": "url_to_post"
        },
        "read": false,
        "created_at": "2025-07-30T14:00:00Z",
        "delivered_at": "2025-07-30T14:00:02Z"
    }
]
```

</details>

### 📣 Send a Broadcast Notification

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {
      "title": "Bodhveda v2 is live!",
      "message": "Check out what is new in our latest release."
    }
    "channel": "announcements",
    "topic": "product",
    "event": "new_feature",
  }'
```

💡 Since no recipient is specified, this is treated as a **broadcast**.

Bodhveda uses the provided `channel`, `topic`, and `event` to:

-   Discover all eligible recipients who have not muted or unsubscribed
-   Respect each recipient’s preferences
-   Materialize notifications lazily in the background

This allows you to reach thousands of users and know exactly who received, who read, and who opened the notification.

**NOTE:** If you omit all `channel`, `topic` and `event`, then this broadcast will materialize as a notification for ALL recipients in your app.

## 🧩 Features

-   ✅ **Targeted Notifications**
    Send 1:1 transactional notifications like password changes, invoice alerts, or DMs directly to known recipients.

-   ✅ **Broadcast Notifications**
    Reach all subscribed or eligible recipients. Supports lazy materialization, respecting mutes and preferences.

-   ✅ **Channel / Topic / Event structure**
    Organize notifications with semantic metadata to enable grouping, filtering, preferences, and analytics.

-   ✅ **Recipient Preferences, Mutes, Subscriptions**
    Allow users to opt into or out of specific types of notifications — even at fine-grained event levels.

-   ✅ **Inbox API**
    Fetch notifications, track read/unread status, delete items — just like a modern inbox.

-   ✅ **Analytics & Observability**
    Get built-in visibility into delivery: see who received a broadcast, who read it, how many opened it — right from Bodhveda's web dashboard.

-   ✅ **Logs Explorer**
    Inspect delivery attempts, failures, and system logs with powerful filtering and traceability.

-   ✅ **Notification & Broadcast Management**
    Browse, manage, and even send new broadcasts directly from the admin dashboard UI.

-   ✅ **Self-hostable or Managed**
    Host it yourself under or use our managed [Bodhveda Cloud](https://bodhveda.com/).

-   ✅ **REST-first Interface**
    Designed to be easily integrated with any backend stack or frontend UI via HTTP REST APIs. We strongly recommend using our in-house SDKs. They simplify the integration process and make it incredibly easy to get started with Bodhveda.

## ❓ Why a Backend, not a library?

Some ask, _"Why isn't this just a library?"_

Because:

-   Delivering notifications at scale is a **stateful problem**: read/unread, retries, preferences, jobs
-   Preferences and subscriptions require **persistent storage and matching logic**
-   Materialization of broadcasts can involve **fanout to thousands**
-   You want analytics and delivery visibility — not just send-and-forget

Bodhveda is **your backend**, with batteries included — but can be self-hosted and used like a microservice.

## 📜 License

[AGPL v3](LICENSE) because notifications should be free to own, run, and customize.

<p align="center">
  Built with 💙 by <a href="https://mudgallabs.com" target="_blank">Mudgal Labs</a>
</p>
