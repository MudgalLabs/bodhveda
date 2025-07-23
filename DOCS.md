# Documentation

### REST API Reference (`rest.md`)

All SDKs use this core HTTP REST API underneath.

Base URL:

```txt
https://api.bodhveda.com/v1
```

Authentication:
Use `Authorization: Bearer <API_KEY>` header with all requests.

---

#### `POST /v1/direct`

Send a direct notification to a recipient.

```json
{
  "recipient": "user_123",
  "payload": { ... }  // max 16KB JSON
}
```

#### `POST /v1/broadcast`

Send a broadcast notification (lazy, only materialized on fetch).

```json
{
  "payload": { ... }
}
```

#### `POST /v1/inbox`

Fetch inbox for a recipient.

```json
{
    "recipient": "user_123",
    "limit": 20,
    "cursor": "abc123"
}
```

Response:

```json
{
  "notifications": [...],
  "nextCursor": "abc123"
}
```

#### `PATCH /v1/read`

Mark specific notifications as read.

```json
{
    "recipient": "user_123",
    "notificationIds": ["n1", "n2"]
}
```

#### `PATCH /v1/read/all`

Mark all notifications for a recipient as read.

```json
{
    "recipient": "user_123"
}
```

#### `DELETE /v1/delete`

Delete specific notifications for a recipient.

```json
{
    "recipient": "user_123",
    "notificationIds": ["n1", "n2"]
}
```

#### `DELETE /v1/delete/all`

Delete all notifications for a recipient.

```json
{
    "recipient": "user_123"
}
```

#### `POST /v1/unread-count`

Returns unread count for a recipient.

```json
{
    "recipient": "user_123"
}
```

Response:

```json
{
    "count": 3
}
```

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
