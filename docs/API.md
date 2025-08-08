# Bodhveda Developer API Reference

Welcome to the Bodhveda Developer API documentation.

Bodhveda is a modern notification infrastructure built for product teams who value thoughtful delivery, user preferences, and a clean developer experience. With just a few API calls, you can:

-   Send direct or broadcast notifications
-   Create and manage recipients (your users)
-   Allow recipients to subscribe or unsubscribe from specific notification types
-   Fetch, mark as read/open/unread, or delete recipient’s notifications
-   Automatically respect recipient's preferences and collect delivery analytics

This guide covers the versioned REST API (v1), designed for both server and client integrations. Whether you're using our SDKs or calling APIs directly, this documentation will walk you through every API, clearly and efficiently.

Base URL:

```
https://api.bodhveda.com/v1
```

## Index

-   [Authentication](#authentication)
-   [Notifications (Full Scope)](#notifications-full-scope)
    -   [Send Notification](#send-notification)
    -   [Targeting Rules](#targeting-rules)
-   [Recipients](#recipients)
    -   [Recipient Notifications (Recipient or Full Scope)](#recipient-notifications-recipient-or-full-scope)
        -   [Get Notifications](#get-notifications)
        -   [Get Unread Count](#unread-count)
        -   [Mark Notifications as Read](#mark-notifications-as-read)
        -   [Mark All as Read](#mark-all-as-read)
        -   [Mark Notifications as Opened](#mark-notifications-as-opened)
        -   [Mark All as Opened](#mark-all-as-opened)
        -   [Mark Notifications as Unread](#mark-notifications-as-unread)
        -   [Delete Notifications](#delete-notifications)
        -   [Delete All Notifications](#delete-all-notifications)
    -   [Recipient Preferences (Recipient or Full Scope)](#recipient-preferences-recipient-or-full-scope)
        -   [Get Global Preferences](#get-global-preferences)
        -   [Subscribe](#subscribe-to-notifications)
        -   [Unsubscribe](#unsubscribe-from-notifications)
    -   [Recipient Management (Full Scope)](#recipient-management-full-scope)
        -   [Get Recipient](#get-recipient)
        -   [Create Recipient](#create-recipient)
        -   [Create Recipients Batch](#create-recipients-batch)
        -   [Update Recipient](#update-recipient)
        -   [Delete Recipient](#delete-recipient)

## Authentication

All requests require the `Authorization` header with a Bearer token:

```
Authorization: Bearer YOUR_API_KEY
```

Replace `YOUR_API_KEY` with your actual API key.

There are 2 scopes for an API KEY :

-   **full** – Full access to all project-level APIs. Use only on the **server side**.
-   **recipient** – Limited-scope API keys intended for **client-side/browser** use. These allow a recipient to **fetch and manage only their own notifications and preferences**.

> ⚠️ **Security Warning:**  
> If your `recipient_id` values are predictable (e.g., auto-incrementing integers), you **should not** expose `recipient`-scoped API keys on the frontend.  
> Instead, route requests through your own backend to ensure recipient access is properly scoped and protected.

---

## Notifications (Full Scope)

> **Requires API key with scope `"full"`**

### Send Notification

**POST** `/v1/notifications/send`

Send a direction notification to a recipient or broadcast it to multiple recipients.

💡 See [Targeting Rules](#targeting-rules) for how to construct the `to` object and how preferences/analytics behave.

**Request Body Example:**

```json
{
    "to": {
        "recipient_id": "user@example.com", // Omit this field to send a broadcast.
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    },
    "payload": {
        "title": "Hey there",
        "message": "You looking good tonight. Slay!!!"
    }
}
```

**Responses:**

-   Direct success: Notification sent to the recipient.
-   Direct rejected: Did not deliver because recipient has unsubscribed to this target.
-   Broadcast success: Broadcast sent to all recipients subscribed to this target.

## Targeting Rules

Bodhveda uses the `to` object to determine who receives the notification and under what context. This applies to both direct and broadcast notifications.

### Direct Notification (No Target)

You can send a notification to a single recipient **without any target**:

```json
{
    "to": {
        "recipient_id": "user@example.com"
    },
    "payload": {
        "title": "Hello!",
        "message": "This won't track preferences or analytics."
    }
}
```

⚠️ Preferences and analytics will not apply.

---

### Direct Notification (With Target)

```json
{
    "to": {
        "recipient_id": "user@example.com",
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    },
    "payload": {
        "title": "Feature Alert",
        "message": "We just launched something cool!"
    }
}
```

-   ✅ Preferences will be respected.
-   ✅ Analytics will track this event.
-   ⚠️ If any of channel, topic, or event is provided, then **all three must be provided**.

---

### Broadcast Notification

```json
{
    "to": {
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    },
    "payload": {
        "title": "New Feature!",
        "message": "Check out what we've just shipped."
    }
}
```

-   ✅ recipient_id must be omitted
-   ✅ channel, topic, and event are required

This sends the notification to **all recipients** who have **subscribed** to the given `channel:topic:event`.

---

### `any` and `none` in Preferences

When subscribing/unsubscribing preferences:

-   `"topic": "any"` means "all topics under this channel/event"
-   `"topic": "none"` means "only the base channel/event"

For example, this preference:

```json
{ "channel": "marketing", "topic": "any", "event": "update" }
```

will match:

-   `marketing:pricing:update`
-   `marketing:feature:update`

However, when sending a notification, `"any"` makes no sense. You must specify a concrete `channel`, `topic`, and `event`.

## Recipients

### Recipient Notifications (Recipient or Full Scope)

> **Requires API key with scope `"recipient"` or `"full"`**

#### Get Notifications

**GET** `/v1/recipients/:recipient/notifications?before=notif_123&limit=20`

Get notifications for a recipient with pagination.

**Path Parameter:**

-   `:recipient` — recipient ID (e.g., user_id, email)

**Query Parameters:**

-   `before` cursor (default: '')
-   `limit` (default: 20)

---

#### Unread Count

**GET** `/v1/recipients/:recipient/notifications/unread-count`

Get the count of unread notifications for a recipient.

**Path Parameter:**

-   `:recipient` — recipient ID (e.g., user_id, email)

**Response Example:**

```json
{
    "unread_count": 5
}
```

---

#### Mark Notifications as Read

**PATCH** `/v1/recipients/:recipient/notifications/mark-read`

**Request Body:**

```json
{
    "notification_ids": ["notif_123", "notif_456"]
}
```

---

#### Mark All as Read

**PATCH** `/v1/recipients/:recipient/notifications/mark-all-read`

Marks all notifications as read for the recipient.

---

#### Mark Notifications as Opened

**PATCH** `/v1/recipients/:recipient/notifications/mark-opened`

**Request Body:**

```json
{
    "notification_ids": ["notif_123", "notif_456"]
}
```

---

#### Mark All as Opened

**PATCH** `/v1/recipients/:recipient/notifications/mark-all-opened`

Marks all notifications as opened for the recipient.

---

#### Mark Notifications as Unread

**PATCH** `/v1/recipients/:recipient/notifications/mark-unread`

**Request Body:**

```json
{
    "notification_ids": ["notif_123", "notif_456"]
}
```

---

#### Delete Notifications

**DELETE** `/v1/recipients/:recipient/notifications/delete`

**Request Body:**

```json
{
    "notification_ids": ["notif_123", "notif_456"]
}
```

---

#### Delete All Notifications

**DELETE** `/v1/recipients/:recipient/notifications/delete-all`

Deletes all notifications for the recipient.

---

### Recipient Preferences (Recipient or Full Scope)

> **Requires API key with scope `"recipient"` or `"full"`**

#### Get Global Preferences

**GET** `/v1/recipients/:recipient/preferences`

Get global notification preferences for a recipient.

---

#### Subscribe to Notifications

**PATCH** `/v1/recipients/:recipient/preferences/subscribe`

**Request Body:**

```json
{
    "to": {
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    }
}
```

---

#### Unsubscribe from Notifications

**PATCH** `/v1/recipients/:recipient/preferences/unsubscribe`

**Request Body:**

```json
{
    "from": {
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    }
}
```

---

### Recipient Management (Full Scope)

> **Requires API key with scope `"full"`**

#### Get Recipient

**GET** `/v1/recipients/:recipient`

Get recipient details.

---

#### Create Recipient

**POST** `/v1/recipients`

**Request Body:**

```json
{
    "recipient_id": "user@example.com",
    "name": "User Name"
}
```

---

#### Create Recipients Batch

**POST** `/v1/recipients/batch`

**Request Body:**

```json
{
    "recipients": [
        { "recipient_id": "user1@example.com", "name": "User One" },
        { "recipient_id": "user2@example.com", "name": "User Two" }
    ]
}
```

---

#### Update Recipient

**PATCH** `/v1/recipients/:recipient`

**Request Body:**

```json
{
    "name": "New Name"
}
```

---

#### Delete Recipient

**DELETE** `/v1/recipients/:recipient`

Deletes a recipient.
