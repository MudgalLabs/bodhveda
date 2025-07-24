# Bodhveda REST API Reference

Base URL:

```http
https://api.bodhveda.com/v1
```

Authentication:
All requests require the `Authorization` header with a Bearer token:

```http
Authorization: Bearer YOUR_API_KEY
```

---

## 📬 Notifications

### Send Direct Notification

**POST** `/v1/recipients/{recipient}/notifications`

Send a direct notification to a specific recipient.

#### Request Body

```json
{
    "payload": {
        "title": "Hi!",
        "type": "info"
    }
}
```

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "direct notification sent",
    "data": {
        "id": "UUID",
        "project_id": "UUID",
        "recipient": "user_123",
        "broadcast_id": null,
        "payload": { "title": "Hi!", "type": "info" },
        "read_at": null,
        "created_at": "ISO_TIMESTAMP",
        "expires_at": "ISO_TIMESTAMP"
    }
}
```

---

### Fetch Notifications (Inbox)

**GET** `/v1/recipients/{recipient}/notifications`

Returns paginated list of notifications for a recipient.

#### Query Parameters

-   `limit` (optional, default: 20)
-   `offset` (optional, default: 0)

#### Success Response

```json
{
  "status": "success",
  "status_code": 200,
  "message": "",
  "data": {
    "notifications": [ ... ],
    "total": 3
  }
}
```

---

### Get Unread Count

**GET** `/v1/recipients/{recipient}/notifications/unread-count`

Returns count of unread notifications.

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "",
    "data": {
        "count": 3
    }
}
```

---

### Mark Notifications as Read

**POST** `/v1/recipients/{recipient}/notifications/read`

Mark one or more notifications as read.

#### Request Body

```json
{
    "ids": ["notif_1", "notif_2"]
}
```

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "notifications marked as read"
}
```

---

### Mark All Notifications as Read

**POST** `/v1/recipients/{recipient}/notifications/read/all`

Marks all notifications for this recipient as read.

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "all notifications marked as read",
    "data": {
        "count": 1
    }
}
```

---

### Delete Notifications

**DELETE** `/v1/recipients/{recipient}/notifications`

Delete one or more notifications by ID.

#### Request Body

```json
{
    "ids": ["notif_1", "notif_2"]
}
```

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "notifications deleted"
}
```

---

### Delete All Notifications

**DELETE** `/v1/recipients/{recipient}/notifications/all`

Deletes **all** notifications for a recipient.

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "all notifications deleted",
    "data": {
        "count": 2
    }
}
```

---

## 📢 Broadcasts

### Send Broadcast Notification

**POST** `/v1/broadcasts`

Send a broadcast notification. It gets materialized into a notification only when a recipient fetches their inbox.

#### Request Body

```json
{
    "payload": {
        "system": true,
        "message": "Server restart"
    }
}
```

#### Success Response

```json
{
  "status": "success",
  "status_code": 200,
  "message": "broadcast notification sent",
  "data": {
    "id": "UUID",
    "project_id": "UUID",
    "payload": { ... },
    "created_at": "ISO_TIMESTAMP",
    "expires_at": "ISO_TIMESTAMP"
  }
}
```

---

### Fetch All Broadcasts

**GET** `/v1/broadcasts`

#### Query Parameters

-   `limit` (optional)
-   `offset` (optional)

#### Success Response

```json
{
  "status": "success",
  "status_code": 200,
  "message": "",
  "data": {
    "broadcasts": [ ... ],
    "total": 1
  }
}
```

---

### Delete Broadcasts

**DELETE** `/v1/broadcasts`

#### Request Body

```json
{
    "ids": ["broadcast_1"]
}
```

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "broadcasts deleted"
}
```

---

### Delete All Broadcasts

**DELETE** `/v1/broadcasts/all`

Deletes **all** broadcasts for your project.

#### Success Response

```json
{
    "status": "success",
    "status_code": 200,
    "message": "all broadcasts deleted",
    "data": {
        "count": 2
    }
}
```
