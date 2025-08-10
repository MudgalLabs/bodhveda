# Bodhveda Developer API Reference

Welcome to the **Bodhveda Developer API** documentation.

## Index

-   [Authentication](#authentication)
-   [Response Structure](#response-structure)
-   [Pagination](#pagination)
-   [Rate Limiting](#rate-limiting)
-   [HTTP Status Codes](#http-status-codes)
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
        -   [Subscribe or Unsubscribe a target](#subscribe-or-unsubscribe-a-target)
        -   [Check subscription to a target](#check-subscription-to-a-target)
    -   [Recipient Management (Full Scope)](#recipient-management-full-scope)
        -   [Get Recipient](#get-recipient)
        -   [Create Recipient](#create-recipient)
        -   [Create Recipients Batch](#create-recipients-batch)
        -   [Update Recipient](#update-recipient)
        -   [Delete Recipient](#delete-recipient)
-   [Support & Contact](#support--contact)

## Base URL

```
https://api.bodhveda.com/v1
```

-   All endpoints in this documentation are relative to the `/v1` path.
-   For testing, a sandbox or staging environment may be available upon request.

## Authentication

All requests require the `Authorization` header with a Bearer token:

```
Authorization: Bearer YOUR_API_KEY
```

Replace `YOUR_API_KEY` with your actual API key.

### API Key Scopes

There are 2 scopes for an API KEY:

-   **full** – Full access to all project-level APIs. Use only on the **server side**.
-   **recipient** – Limited-scope API keys intended for **client-side/browser** use. These allow a recipient to **fetch and manage only their own notifications and preferences**.

> ⚠️ **Security Warning:**  
> If your `recipient_id` values are predictable (e.g., auto-incrementing integers), you **should not** expose `recipient`-scoped API keys on the frontend.  
> Instead, route requests through your own backend to ensure recipient access is properly scoped and protected.

### Authentication Errors

-   If the API key is missing or invalid, you will receive a `401 Unauthorized` response.
-   If the API key does not have sufficient permissions, you will receive a `403 Forbidden` response.

## Response Structure

All responses follow this structure:

```json
{
    "status": "success" // or "error"
    "status_code": 200,
    "message": "Human readable message",
    "errors": [ ], // array of error objects, if any
    "data": { }, // or [ ] response data, if any
}
```

#### Error Object

```json
{
    "message": "Short error message",
    "description": "Detailed technical description",
    "property_path": "field_name",
    "invalid_value": "the value that caused the error"
}
```

#### Example Error Response

```json
{
    "status": "error",
    "status_code": 400,
    "message": "Invalid request payload",
    "errors": [
        {
            "message": "Recipient ID is required",
            "description": "Recipient ID cannot be empty",
            "property_path": "recipient_id",
            "invalid_value": ""
        }
    ]
}
```

## Rate Limiting

-   The API enforces rate limits to prevent abuse.
-   If you exceed the allowed rate, you will receive a `429 Too Many Requests` response.
-   Standard limit: 100 requests per minute for a project (subject to change).

## HTTP Status Codes

-   `200 OK` – Successful GET/PUT/PATCH/DELETE
-   `201 Created` – Successful POST (resource created)
-   `400 Bad Request` – Invalid input or malformed request
-   `401 Unauthorized` – Missing or invalid API key
-   `403 Forbidden` – Insufficient permissions
-   `404 Not Found` – Resource does not exist
-   `409 Conflict` – Resource already exists
-   `429 Too Many Requests` – Rate limit exceeded
-   `500 Internal Server Error` – Unexpected server error

## Notifications (Full Scope)

> **Requires API key with scope `"full"`**

### Send Notification

**POST** `/v1/notifications/send`

Send a direction notification to a recipient or broadcast it to multiple recipients.

💡 See [Targeting Rules](#targeting-rules) for how to construct the `to` object and how preferences and analytics work.

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

However, when sending a notification, `"any"` makes no sense, although `"none"` topic is allowed. You must specify a concrete `channel`, `topic`, and `event`.

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

#### Mark Notifications as Read

**PATCH** `/v1/recipients/:recipient/notifications/mark-read`

**Request Body:**

```json
{
    "notification_ids": [123, 456]
}
```

**Response Example:**

```json
{
    "updated": 5
}
```

#### Mark All as Read

**PATCH** `/v1/recipients/:recipient/notifications/mark-all-read`

Marks all notifications as read for the recipient.

**Response Example:**

```json
{
    "updated": 42
}
```

#### Mark Notifications as Unread

**PATCH** `/v1/recipients/:recipient/notifications/mark-unread`

**Request Body:**

```json
{
    "notification_ids": [123, 456]
}
```

**Response Example:**

```json
{
    "updated": 2
}
```

#### Mark Notifications as Opened

**PATCH** `/v1/recipients/:recipient/notifications/mark-opened`

**Request Body:**

```json
{
    "notification_ids": [123, 456]
}
```

**Response Example:**

```json
{
    "updated": 2
}
```

#### Mark All as Opened

**PATCH** `/v1/recipients/:recipient/notifications/mark-all-opened`

Marks all notifications as opened for the recipient.

**Response Example:**

```json
{
    "updated": 69
}
```

#### Delete Notifications

**DELETE** `/v1/recipients/:recipient/notifications/delete`

**Request Body:**

```json
{
    "notification_ids": [123, 456]
}
```

**Response Example:**

```json
{
    "updated": 2
}
```

#### Delete All Notifications

**DELETE** `/v1/recipients/:recipient/notifications/delete-all`

Deletes all notifications for the recipient.

**Response Example:**

```json
{
    "updated": 13
}
```

### Recipient Preferences (Recipient or Full Scope)

> **Requires API key with scope `"recipient"` or `"full"`**

#### Get Global Preferences

**GET** `/v1/recipients/:recipient/preferences/global`

Get recipient's preference to project/app level defined preferences.

**Response Body:**

```json
{
    "global_preferences": [
        {
            "target": {
                "channel": "announcements",
                "topic": "none",
                "event": "new_feature",
                "label": "New feature release"
            },
            "state": {
                "subscribed": true,
                "inherited": true
            }
        },
        {
            "target": {
                "channel": "posts",
                "topic": "any",
                "event": "new_comment",
                "label": "Comment on your post"
            },
            "state": {
                "subscribed": true,
                "inherited": true
            }
        },
        {
            "target": {
                "channel": "posts",
                "topic": "any",
                "event": "new_like",
                "label": "Like on your post"
            },
            "state": {
                "subscribed": false,
                "inherited": false
            }
        }
    ]
}
```

#### Subscribe or Unsubscribe a target

**PATCH** `/v1/recipients/:recipient/preferences/targets`

**Request Body:**

```json
{
    "target": {
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    },
    "state": {
        "subscribed": true // or `false` for unsubscribing
    }
}
```

**Response Body:**

```json
{
    "target": {
        "channel": "announcements",
        "topic": "none",
        "event": "new_feature"
    },
    "state": {
        "subscribed": true,
        "inherited": false // `true` means it was derived from project's global default and not explicitly set by the recipient.
    }
}
```

#### Check subscription to a target

**GET** `/v1/recipients/:recipient/preferences/targets?channel=posts&topic=post_id_123&event=new_comment`

**Response Body:**

```json
{
    "target": {
        "channel": "posts",
        "topic": "post_id_123",
        "event": "new_comment"
    },
    "state": {
        "subscribed": true,
        "inherited": false
    }
}
```

### Recipient Management (Full Scope)

> **Requires API key with scope `"full"`**

#### Get Recipient

**GET** `/v1/recipients/:recipient`

Get recipient details.

**Response Body:**

```json
{
    "status": "success",
    "status_code": 200,
    "message": "",
    "data": {
        "recipient_id": "ceoshikhar@gmail.com",
        "name": "Shikhar Sharma",
        "created_at": "2025-08-04T15:45:15.342589+05:30",
        "updated_at": "2025-08-04T15:45:15.342589+05:30"
    }
}
```

#### Create Recipient

**POST** `/v1/recipients`

**Request Body:**

```json
{
    "recipient_id": "user@example.com",
    "name": "Recipient Name"
}
```

**Response Body:**

```json
{
    "status": "success",
    "status_code": 201,
    "message": "Recipient created",
    "data": {
        "recipient_id": "user@example.com",
        "name": "Recipient Name",
        "created_at": "2025-08-09T17:18:58.187014+05:30",
        "updated_at": "2025-08-09T17:18:58.187014+05:30"
    }
}
```

#### Create Recipients Batch

**POST** `/v1/recipients/batch`

**Request Body:**

```json
{
    "recipients": [
        {
            "recipient_id": "ceoshikhar@gmail.com",
            "name": "Shikhar Sharma"
        },
        {
            "recipient_id": "shikar.sharma@gmail.com",
            "name": "Shikhar Sharma Fake"
        },
        {
            "name": "Shikhar Sharma Fake"
        }
    ]
}
```

**Response Body:**

```json
{
    "status": "success",
    "status_code": 201,
    "message": "",
    "data": {
        "created": [],
        "updated": [
            {
                "recipient_id": "ceoshikhar@gmail.com"
            },
            {
                "recipient_id": "shikar.sharma@gmail.com"
            }
        ],
        "failed": [
            {
                "errors": [
                    {
                        "message": "Recipient ID is required",
                        "description": "Recipient ID cannot be empty",
                        "property_path": "recipient_id",
                        "invalid_value": ""
                    }
                ],
                "recipient_id": "",
                "batch_index": 2
            }
        ]
    }
}
```

#### Update Recipient

**PATCH** `/v1/recipients/:recipient`

Update the details of a recipient.

**Path Parameter:**

-   `:recipient` — recipient ID (e.g., user_id, email)

**Request Body:**

```json
{
    "name": "New Name"
}
```

**Response Body:**

```json
{
    "status": "success",
    "status_code": 200,
    "message": "Recipient updated",
    "data": {
        "recipient_id": "user@example.com",
        "name": "New Name",
        "created_at": "2025-08-09T17:18:58.187014+05:30",
        "updated_at": "2025-08-10T12:34:56.123456+05:30"
    }
}
```

#### Delete Recipient

**DELETE** `/v1/recipients/:recipient`

Deletes a recipient.

**Path Parameter:**

-   `:recipient` — recipient ID (e.g., user_id, email)

**Response Body:**

```json
{
    "status": "success",
    "status_code": 200,
    "message": "Recipient deleted"
}
```

## Support & Contact

-   For help, bug reports, or feature requests, contact [support@bodhveda.com](mailto:support@bodhveda.com).
