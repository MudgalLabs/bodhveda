# ⚡ Quickstart: Send Your First Notification

Start sending notifications with Bodhveda in minutes using the REST API.

## Direct Notification

Send a notification to a [recipient](./core-concepts.md#recipient-model) in just 3 steps. If the recipient doesn't exist, Bodhveda creates it automatically when sending a direct notification.

### 1. Get you API key.

-   Go to [Bodhveda Console](https://console.bodhveda.com).
-   Create a **new project**.
-   Click on **API Keys** in the sidebar.
-   Generate an API key with `full` scope by clicking on **Create API Key**.

### 2. Send a direct notification to the recipient.

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "recipient_id": "recipient_123" },
    "payload": { "title": "Welcome!", "message": "Thanks for joining." }
  }'
```

### 3. Show notifications to the recipient in their "Inbox".

Fetch notifications for a recipient:

```bash
curl -X GET "https://api.bodhveda.com/v1/recipients/recipient_123/notifications" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

```json
[
    {
        "id": 42069,
        "recipient_id": "recipient_123",
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
2. Send a direct notification by clicking on **Send Notification**.

## Broadcast Notification

Broadcast to multiple recipients. This requires some setup:

-   At least one [recipient](./core-concepts.md#recipient-model) must exist.
-   At least one [project preference](./core-concepts.md#preferences) must exist.

### 1. Add a recipient.

```bash
curl -X POST https://api.bodhveda.com/v1/recipients \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"recipient_id": "user@example.com", "name": "Jane Doe"}'
```

### 2. Add a project preference.

Before broadcasting, you must add at least one project preference to define a target (channel, topic, event):

1. Go to [Bodhveda Console](https://console.bodhveda.com).
2. Click on **Preferences** in the sidebar.
3. Click on **Create Preference**.
4. Define a target by mentioning the channel, topic, and event, give it a label, and set default to enabled.
5. Save the preference.

This way all recipients are by default subscribed to this target in your project.

### 3. Send a Broadcast Notification

```bash
curl -X POST https://api.bodhveda.com/v1/notifications/send \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": { "channel": "announcements", "topic": "product", "event": "new_feature" },
    "payload": { "title": "Big News!", "message": "We just launched a new feature." }
  }'
```

💡 No `recipient_id`, makes this notification turn into a broadcast. All recipients subscribed to this target will recieve this notification.

> [Read more](./core-concepts.md#notification-targeting) for details on channels, topics, events, and preferences.

## Next Steps

-   [Explore the API reference](./api-reference.md)
-   [Learn about core concepts](./core-concepts.md)

> **Tip:** Prefer using the [SDK](./api-reference.md#sdk) if available for your language for easier integration.
