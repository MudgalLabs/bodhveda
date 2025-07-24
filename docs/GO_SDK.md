# Bodhveda Go SDK

The official Go SDK for [Bodhveda](https://bodhveda.com).

---

## Table of Contents

-   [Installation](#installation)
-   [Initialization](#initialization)
-   [Types](#types)

    -   [Payload](#payload)
    -   [Notification](#notification)
    -   [Broadcast](#broadcast)

-   [Error Handling](#error-handling)
-   [Notifications](#notifications)

    -   [Send](#send)
    -   [Fetch](#fetch)
    -   [UnreadCount](#unreadcount)
    -   [MarkAsRead](#markasread)
    -   [MarkAllAsRead](#markallasread)
    -   [Delete](#delete)
    -   [DeleteAll](#deleteall)

-   [Broadcasts](#broadcasts)

    -   [Send](#send-1)
    -   [Fetch](#fetch-1)
    -   [Delete](#delete-1)
    -   [DeleteAll](#deleteall-1)

-   [License](#license)

---

## Installation

Install the SDK using Go modules:

```bash
go get github.com/mudgallabs/bodhveda
```

---

## Initialization

```go
import "github.com/mudgallabs/bodhveda"

bodhveda := bodhveda.New("YOUR_API_KEY")
```

> Replace `YOUR_API_KEY` with your actual Bodhveda API key. You can generate and manage your API keys from the **API Keys** tab in the [dashboard](https://web.bodhveda.com/api-keys).

---

## Types

### Payload

```go
// Payload must be any valid JSON-serializable value (excluding null)
type Payload = any
```

### Notification

```go
type Notification struct {
    ID          string     `json:"id"`
    ProjectID   string     `json:"projectId"`
    Recipient   string     `json:"recipient"`
    BroadcastID *string    `json:"broadcastId,omitempty"`
    Payload     Payload    `json:"payload"`
    ReadAt      *time.Time  `json:"readAt,omitempty"`
    CreatedAt   time.Time   `json:"createdAt"`
    ExpiresAt   time.Time   `json:"expiresAt"`
}
```

### Broadcast

```go
type Broadcast struct {
    ID        string   `json:"id"`
    ProjectID string   `json:"projectId"`
    Payload   Payload  `json:"payload"`
    CreatedAt time.Time   `json:"createdAt"`
    ExpiresAt time.Time   `json:"expiresAt"`
}
```

---

## Error Handling

```go
notif, err := bodhveda.Notifications.Send("recipient_123", map[string]interface{}{
    "title": "Welcome!",
    "type":  "info",
})
if err != nil {
    log.Fatalf("failed to send: %v", err)
}
```

---

## Notifications

### Send

```go
Send(recipient string, payload interface{}) (*Notification, error)
```

```go
notif, err := bodhveda.Notifications.Send("recipient_123", map[string]interface{}{
    "title": "Welcome!",
    "type":  "info",
})
```

### Fetch

```go
Fetch(recipient string) ([]Notification, error)
```

```go
notifs, err := bodhveda.Notifications.Fetch("recipient_123")
```

### UnreadCount

```go
UnreadCount(recipient string) (int, error)
```

```go
count, err := bodhveda.Notifications.UnreadCount("recipient_123")
```

### MarkAsRead

```go
MarkAsRead(recipient string, ids []string) error
```

```go
err := bodhveda.Notifications.MarkAsRead("recipient_123", []string{"id1", "id2"})
```

### MarkAllAsRead

```go
MarkAllAsRead(recipient string) (int, error)
```

```go
count, err := client.Notifications.MarkAllAsRead("recipient_123")
```

### Delete

```go
Delete(recipient string, ids []string) error
```

```go
err := bodhveda.Notifications.Delete("recipient_123", []string{"id1", "id2"})
```

### DeleteAll

```go
DeleteAll(recipient string) (int, error)
```

```go
count, err := client.Notifications.DeleteAll("recipient_123")
```

---

## Broadcasts

### Send

```go
Send(payload interface{}) (*Broadcast, error)
```

```go
broadcast, err := client.Broadcasts.Send(map[string]interface{}{
    "system": true,
    "message": "Server restart at midnight.",
})
```

### Fetch

```go
Fetch() ([]Broadcast, error)
```

```go
broadcasts, err := client.Broadcasts.Fetch()
```

### Delete

```go
Delete(ids []string) error
```

```go
err := client.Broadcasts.Delete([]string{"broadcast_id_1"})
```

### DeleteAll

```go
DeleteAll() (int, error)
```

```go
count, err := client.Broadcasts.DeleteAll()
```

---

## License

This SDK is licensed under the MIT License.

---

For more information, visit [bodhveda.com](https://bodhveda.com) or contact support.
