# Go SDK for Bodhveda

Official Go SDK for Bodhveda.

It offers a simpler way to work with Bodhveda APIs.

## Index

-   [Installation](#installation)
-   [Quick Start](#quick-start)
-   [Notifications](#notifications)
-   [Recipients](#recipients)
    -   [Recipient Notifications](#recipient-notifications)
    -   [Recipient Preferences](#recipient-preferences)
-   [License](#license)

## Installation

```bash
go get github.com/MudgalLabs/bodhveda/sdk/go
```

## Quick Start

```go
import (
    "context"
    "encoding/json"

    bodhveda "github.com/MudgalLabs/bodhveda/sdk/go"
)

ctx := context.Background()
client := bodhveda.NewClient("YOUR_API_KEY", nil)

// Send a notification to a recipient.
// Note: Bodhveda will create the recipient if it does not already exist.
recipientID := "user-123"
payload, _ := json.Marshal(map[string]any{"message": "Hello, world!"})
resp, _ := client.Notifications.Send(ctx, &bodhveda.SendNotificationRequest{
    RecipientID: &recipientID,
    Payload:     payload,
})

// List all notifications for a recipient.
resp, _ := client.Recipients.Notifications.List(ctx, "user-123", nil)
// resp.Notifications
```

## Notifications

### Send a notification

Send a notification to a recipient or broadcast to a target.

```go
import "encoding/json"

recipientID := "user-123"
payload, _ := json.Marshal(map[string]any{"message": "Hello, world!"})
resp, _ := client.Notifications.Send(ctx, &bodhveda.SendNotificationRequest{
    RecipientID: &recipientID,
    Payload:     payload,
})
// resp.Notification or resp.Broadcast
```

---

## Recipients

### Create a recipient

Create a new recipient.

```go
name := "Alice"
resp, _ := client.Recipients.Create(ctx, &bodhveda.CreateRecipientRequest{
    ID: "user-123",
    Name: &name,
})
// resp.Recipient
```

### Create multiple recipients (batch)

Create multiple recipients in a single request.

```go
name1 := "Alice"
name2 := "Bob"
resp, _ := client.Recipients.CreateBatch(ctx, &bodhveda.CreateRecipientsBatchRequest{
    Recipients: []bodhveda.CreateRecipientRequest{
        {ID: "user-1", Name: &name1},
        {ID: "user-2", Name: &name2},
    },
})
// resp.Created, resp.Updated, resp.Failed
```

### Get a recipient

Retrieve details of a recipient by ID.

```go
resp, _ := client.Recipients.Get(ctx, "user-123")
// resp.Recipient
```

### Update a recipient

Update recipient details.

```go
updatedName := "Alice Updated"
resp, _ := client.Recipients.Update(ctx, "user-123", &bodhveda.UpdateRecipientRequest{
    Name: &updatedName,
})
// resp.Recipient
```

### Delete a recipient

Delete a recipient by ID.

```go
err := client.Recipients.Delete(ctx, "user-123")
```

---

## Recipient Notifications

### List notifications

List notifications for a recipient.

```go
resp, _ := client.Recipients.Notifications.List(ctx, "user-123", nil)
// resp.Notifications
```

### Get unread notification count

Get the count of unread notifications for a recipient.

```go
resp, _ := client.Recipients.Notifications.UnreadCount(ctx, "user-123")
// resp.UnreadCount
```

### Update notification state

Update the state (e.g., mark as read) of notifications for a recipient.

```go
read := true
resp, _ := client.Recipients.Notifications.UpdateState(ctx, "user-123", &bodhveda.UpdateNotificationsStateRequest{
    IDs: []int{1, 2, 3},
    State: bodhveda.NotificationStateOptional{
        Read: &read,
    },
})
// resp.UpdatedCount
```

### Delete notifications

Delete notifications for a recipient.

```go
resp, _ := client.Recipients.Notifications.Delete(ctx, "user-123", &bodhveda.DeleteNotificationsRequest{
    IDs: []int{1, 2, 3},
})
// resp.DeletedCount
```

---

## Recipient Preferences

### List preferences

List all preferences for a recipient.

```go
resp, _ := client.Recipients.Preferences.List(ctx, "user-123")
// resp.Preferences
```

### Set a preference

Set a notification preference for a recipient.

```go
resp, _ := client.Recipients.Preferences.Set(ctx, "user-123", &bodhveda.SetPreferenceRequest{
    Target: bodhveda.Target{
        Channel: "email",
        Topic:   "news",
        Event:   "daily",
    },
    State: struct{ Enabled bool }{Enabled: true},
})
// resp.Target, resp.State
```

### Check a preference

Check the state of a specific preference for a recipient.

```go
resp, _ := client.Recipients.Preferences.Check(ctx, "user-123", &bodhveda.CheckPreferenceRequest{
    Target: bodhveda.Target{
        Channel: "email",
        Topic:   "news",
        Event:   "daily",
    },
})
// resp.Target, resp.State
```

---

## License

MIT
