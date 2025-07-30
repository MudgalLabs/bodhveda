# Bodhveda REST API Reference

Base URL:

```
https://api.bodhveda.com/v1
```

Authentication:
All requests require the `Authorization` header with a Bearer token:

```
Authorization: Bearer YOUR_API_KEY
```

# Bodhveda REST API Reference

This document outlines the REST API endpoints available in Bodhveda. These endpoints power the platform's core features: sending and fetching notifications, managing recipient preferences, muting/unmuting, and managing recipients.

Base URL:

```
https://api.bodhveda.com/v1
```

Authentication:
All requests require the `Authorization` header with a Bearer token:

```
Authorization: Bearer YOUR_API_KEY
```

## Index

-   [Send Notification (Direct or Broadcast)](#send-notification-direct-or-broadcast)
-   [Fetch Recipient Inbox](#fetch-recipient-inbox)
-   [Get Unread Count](#get-unread-count)
-   [Mark Notifications as Read](#mark-notifications-as-read)
-   [Mark Notifications as Opened](#mark-notifications-as-opened)
-   [Delete Notifications](#delete-notifications)
-   [Delete All Notifications](#delete-all-notifications)
-   [Create Notification Preference Type](#create-notification-preference-type)
-   [Get Preferences for Recipient](#get-preferences-for-recipient)
-   [Mute Notification Type](#mute-notification-type)
-   [Unmute Notification Type](#unmute-notification-type)
-   [List Mutes for Recipient](#list-mutes-for-recipient)
-   [Create Recipient (Join)](#create-recipient-join)
-   [Get Recipient](#get-recipient)
-   [Update Recipient](#update-recipient)
-   [Delete Recipient](#delete-recipient)
-   [List Recipients](#list-recipients)

## Send Notification (Direct or Broadcast)

**POST** `/v1/notifications`

Unified endpoint for both direct and broadcast notifications.

-   If `recipient` is provided → **Direct Notification** (1:1)
-   If `recipient` is not provided → **Broadcast Notification** (fan-out via background jobs)

```json
{
    "recipient": "user_123",
    "payload": {
        "title": "Someone followed you",
        "body": "Elon Musk just followed your profile."
    },
    "channel": "social",
    "topic": "followers",
    "event": "followed"
}
```

---

## Fetch Recipient Inbox

**GET** `/v1/recipients/{recipient}/notifications`

Query Params:

-   `limit`
-   `offset`

Example Response:

```json
[
    {
        "id": "notif_abc123",
        "payload": {
            "title": "Welcome!",
            "body": "Thanks for signing up."
        },
        "read": false,
        "opened": false,
        "delivered_at": "2025-07-30T10:15:00Z"
    }
]
```

---

## Get Unread Count

**GET** `/v1/recipients/{recipient}/notifications/unread-count`

Example Response:

```json
{
    "unread_count": 3
}
```

---

## Mark Notifications as Read

**POST** `/v1/recipients/{recipient}/notifications/read`

```json
{
    "ids": ["notif_123", "notif_456"]
}
```

---

## Mark Notifications as Opened

**POST** `/v1/recipients/{recipient}/notifications/open`

```json
{
    "ids": ["notif_123"]
}
```

---

## Delete Notifications

**DELETE** `/v1/recipients/{recipient}/notifications`

```json
{
    "ids": ["notif_123"]
}
```

---

## Delete All Notifications

**DELETE** `/v1/recipients/{recipient}/notifications/all`

---

## Create Notification Preference Type

**POST** `/v1/projects/{project_id}/preference-types`

```json
{
    "channel": "announcements",
    "topic": null,
    "event": "new_feature"
}
```

---

## Get Preferences for Recipient

**GET** `/v1/recipients/{recipient}/preferences`

Returns all available preference types + mute state for that recipient.

Example Response:

```json
[
    {
        "channel": "announcements",
        "event": "new_feature",
        "muted": false
    }
]
```

---

## Mute Notification Type

**POST** `/v1/recipients/{recipient}/mutes`

```json
{
    "channel": "product",
    "topic": null,
    "event": "feature_release"
}
```

---

## Unmute Notification Type

**DELETE** `/v1/recipients/{recipient}/mutes`

```json
{
    "channel": "product",
    "topic": null,
    "event": "feature_release"
}
```

---

## List Mutes for Recipient

**GET** `/v1/recipients/{recipient}/mutes`

Example Response:

```json
[
    {
        "channel": "product",
        "event": "feature_release"
    }
]
```

---

## Create Recipient (Join)

**POST** `/v1/recipients`

```json
{
    "recipient": "user_123",
    "name": "Elon Musk",
    "properties": {
        "role": "admin",
        "location": "Mars"
    }
}
```

Example Response:

```json
{
    "recipient": "user_123",
    "name": "Elon Musk",
    "properties": {
        "role": "admin",
        "location": "Mars"
    },
    "created_at": "2025-07-30T14:00:00Z"
}
```

---

## Get Recipient

**GET** `/v1/recipients/{recipient}`

Example Response:

```json
{
    "recipient": "user_123",
    "name": "Elon Musk",
    "properties": {
        "location": "Mars"
    },
    "created_at": "2025-07-30T14:00:00Z"
}
```

---

## Update Recipient

**PATCH** `/v1/recipients/{recipient}`

```json
{
    "name": "Elon Musk Jr.",
    "properties": {
        "location": "Earth"
    }
}
```

---

## Delete Recipient

**DELETE** `/v1/recipients/{recipient}`

---

## List Recipients

**GET** `/v1/recipients`

Supports pagination (limit, offset).

Example Response:

```json
[
    {
        "recipient": "user_123",
        "name": "Elon Musk",
        "properties": {
            "location": "Mars"
        }
    }
]
```

---
