# ⚡ Quickstart: Send Your First Notification

Get started with Bodhveda in minutes.

## Direct Notification

Send a notification to a [recipient](./core-concepts.md#recipient-model) in just 3 steps. If the recipient doesn't exist, Bodhveda creates it automatically when sending a direct notification.

### 1. Get Your API Key

-   Go to [Bodhveda Console](https://console.bodhveda.com).
-   Create a **new project**.
-   Go to **API Keys**.
-   Generate an API key with `full` scope by clicking on **Create API Key**.

### 2. Send a Direct Notification

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "recipient_id": "user@example.com" },
    "payload": { "title": "Welcome!", "message": "Thanks for joining." }
  }'
```

### 3. See Notifications

Fetch notifications for a recipient:

```bash
curl -X GET "https://api.bodhveda.com/v1/recipients/user@example.com/notifications" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

```json
[
    {
        "id": 42069,
        "recipient_id": "user@example.com",
        "payload": {
            "title": "Welcome!",
            "message": "Thanks for joining."
        },
        "broadcast_id": null,
        "channel": "",
        "topic": "",
        "event": "",
        "seen": false,
        "clicked": false,
        "created_at": "2025-08-09T13:51:38.671616+05:30",
        "updated_at": "2025-08-09T13:51:38.671616+05:30"
    }
]
```

## Use the Console (No API Key Needed)

You can do everything above via the [Bodhveda Console](https://console.bodhveda.com):

1. After creating the project, go to **Notifications**.
2. Send a Direct Notification by clicking on **Send Notification**.

## Broadcast Notification

Broadcast to multiple recipients. This requires some setup:

-   [Recipients](./core-concepts.md#recipient-model) must exist.
-   [Global preferences](./core-concepts.md#preferences) should be set (to define default [targets](./core-concepts.md#notification-targeting)).

### 1. Add Recipients

```bash
curl -X POST https://api.bodhveda.com/v1/recipients \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"recipient_id": "user@example.com", "name": "Jane Doe"}'
```

### 2. Add a Global Preference (Required)

Before broadcasting, you must add at least one global preference to define a target (channel, topic, event):

1. Go to the **Preferences** tab in the Console.
2. Click **Create Preference**.
3. Define a target by selecting the channel, topic, and event, give it a friendly label, and set default to enabled.
4. Save the preference.

This way all recipients are by default subscribed to this target in your project.

### 3. Send a Broadcast Notification

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "channel": "announcements", "topic": "none", "event": "new_feature" },
    "payload": { "title": "Big News!", "message": "We just launched a new feature." }
  }'
```

💡 No `recipient_id`, makes this notification turn into a broadcast. All recipients subscribed to this target will recieve this notification.

> See [Core Concepts](./core-concepts.md#notification-targeting) for details on channels, topics, events, and preferences.

## Next Steps

-   [Explore the API Reference](./api-reference.md)
-   [Learn about Core Concepts](./core-concepts.md)

> **Tip:** Prefer using the [SDK](./api-reference.md#sdk) if available for your language for easier integration.
