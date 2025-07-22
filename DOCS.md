# Documentation

### Rest API

All SDKs use this core HTTP REST API underneath.

Base URL:

```txt
https://api.bodhveda.com/v1
```

> All requests must include your API Key in the header:

```
Authorization: Bearer YOUR_API_KEY
```

---

#### `POST /v1/direct`

Send a direct notification to a recipient.

```json
{
  "recipient": "user_123",
  "payload": { ... }  // max 1MB JSON
}
```

#### `POST /v1/broadcast`

Send a broadcast notification (lazy, only materialized on fetch).

```json
{
  "payload": { ... }
}
```

#### `GET /v1/inbox/{recipient}`

Fetch inbox for a recipient.
Query params:

-   `limit` (optional, default 20)
-   `cursor` (optional, for pagination)

Returns:

```json
{
  "notifications": [...],
  "nextCursor": "abc123"
}
```

#### `POST /v1/read`

Mark specific notifications as read.

```json
{
    "recipient": "user_123",
    "notificationIds": ["n1", "n2"]
}
```

#### `POST /v1/read-all`

Mark all notifications for a recipient as read.

```json
{
    "recipient": "user_123"
}
```

#### `GET /v1/unread-count/{recipient}`

Returns unread count for a recipient.

```json
{
    "count": 3
}
```

#### `DELETE /v1/delete`

Delete one or more notifications.

```json
{
    "notificationIds": ["n1", "n2"]
}
```

#### `DELETE /v1/delete-all`

Delete all notifications for a recipient.

```json
{
    "recipient": "user_123"
}
```

---

### Node SDK

Install:

```bash
npm install @bodhveda/sdk
```

Usage:

```ts
import bodhveda from "@bodhveda/sdk";

bodhveda.setup("YOUR_API_KEY");

await bodhveda.direct("user_123", { title: "Hi!", type: "info" });
await bodhveda.broadcast({ system: true, message: "Server restart" });

const inbox = await bodhveda.inbox("user_123");
await bodhveda.read("user_123", ["notif_1"]);
await bodhveda.readAll("user_123");

await bodhveda.delete(["notif_1"]);
await bodhveda.deleteAll("user_123");

const unread = await bodhveda.getUnreadCount("user_123");
```

---

### Go SDK

Install:

```bash
go get github.com/bodhveda/sdk
```

Usage:

```go
import "github.com/bodhveda/sdk"

client := sdk.NewClient("YOUR_API_KEY")

client.Direct("user_123", map[string]any{
    "title": "Hi!",
    "type": "info",
})

client.Broadcast(map[string]any{
    "system": true,
    "message": "Scheduled downtime"
})

inbox, _ := client.Inbox("user_123", nil)
client.Read("user_123", []string{"notif_1"})
client.ReadAll("user_123")

client.Delete([]string{"notif_1"})
client.DeleteAll("user_123")

count, _ := client.UnreadCount("user_123")
```

---

These docs will soon be available on:

```txt
https://bodhveda.com/docs
```

And will include examples, SDK references, and usage patterns.
