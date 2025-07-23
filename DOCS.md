# Documentation

### REST API Reference (`rest.md`)

All SDKs use this core HTTP REST API underneath.

Base URL:

```txt
https://api.bodhveda.com/v1
```

All requests require authentication using the API key:

```
Authorization: Bearer YOUR_API_KEY
```

---

### Direct Notification

**POST** `/v1/recipients/{recipient}/direct`
Send a direct notification to a specific recipient.

```json
{
  "payload": { ... } // max 16KB JSON
}
```

---

### Broadcast Notification

**POST** `/v1/broadcasts`
Send a broadcast notification. Materialized only when recipients fetch inbox.

```json
{
  "payload": { ... } // max 16KB JSON
}
```

---

### Fetch Inbox

**GET** `/v1/recipients/{recipient}/inbox`
Query Params:

-   `limit` (optional, default 20)
-   `offset` (optional, default 0)

Returns:

```json
{
  "notifications": [...],
  "total": 42
}
```

---

### Mark Notifications as Read

**POST** `/v1/recipients/{recipient}/notifications/read`

```json
{
    "ids": ["notif_1", "notif_2"]
}
```

---

### Mark All Notifications as Read

**POST** `/v1/recipients/{recipient}/notifications/read/all`
No payload required.

---

### Delete Notifications

**DELETE** `/v1/recipients/{recipient}/notifications`

```json
{
    "ids": ["notif_1", "notif_2"]
}
```

---

### Delete All Notifications

**DELETE** `/v1/recipients/{recipient}/notifications/all`
No payload required.

---

### Get Unread Count

**GET** `/v1/recipients/{recipient}/notifications/unread-count`
Returns:

```json
{
    "count": 3
}
```

---

These endpoints will be documented at:

```txt
https://bodhveda.com/docs
```

And include examples, SDKs, and usage patterns.

---

### Node SDK (`node.md`)

Install:

```bash
npm install @bodhveda/sdk
```

Usage:

```ts
import bodhveda from "@bodhveda";

bodhveda.init("YOUR_API_KEY");

await bodhveda.direct("user_123", { title: "Hi!", type: "info" });
await bodhveda.broadcast({ system: true, message: "Server restart" });

const inbox = await bodhveda.inbox("user_123");
await bodhveda.read("user_123", ["notif_1"]);
await bodhveda.readAll("user_123");

await bodhveda.delete("user_123", ["notif_1"]);
await bodhveda.deleteAll("user_123");

const unread = await bodhveda.unreadCount("user_123");
```

---

### Go SDK (`go.md`)

Install:

```bash
go get github.com/mudgallabs/bodhveda
```

Usage:

```go
import "github.com/mudgallabs/bodhveda"

client := sdk.NewClient("YOUR_API_KEY")

client.Direct("user_123", map[string]any{
    "title": "Hi!",
    "type": "info",
})

client.Broadcast(map[string]any{
    "system": true,
    "message": "Scheduled downtime",
})

inbox, _ := client.Inbox("user_123", nil)
client.Read("user_123", []string{"notif_1"})
client.ReadAll("user_123")

client.Delete("user_123", []string{"notif_1"})
client.DeleteAll("user_123")

count, _ := client.UnreadCount("user_123")
```

---

These docs will soon be available on:

```txt
https://bodhveda.com/docs
```

And will include examples, SDK references, and usage patterns.
