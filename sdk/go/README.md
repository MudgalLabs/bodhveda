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
    -   [Recipient Contacts](#recipient-contacts)
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

### Send with email

Include the optional `Email` block to also send an email. Its presence makes email
eligible (direct sends only — an email block on a broadcast returns `400`). Bodhveda
does no templating: you render the subject/HTML/text yourself and pass the result.

Email fires only when the `(target, email)` pair is cataloged, the recipient's email
preference is enabled, and the recipient has a primary email
[contact](#recipient-contacts). Per-medium outcomes are returned in `resp.Deliveries`.

```go
recipientID := "user-123"
payload, _ := json.Marshal(map[string]any{"title": "Your daily digest is ready."})
resp, _ := client.Notifications.Send(ctx, &bodhveda.SendNotificationRequest{
    RecipientID: &recipientID,
    Target: &bodhveda.Target{Channel: "digest", Topic: "none", Event: "sent"},
    Payload: payload,
    Email: &bodhveda.EmailContent{
        Subject: "Your daily digest",
        HTML:    "<h1>Your daily digest</h1><p>3 new follow-ups today.</p>",
    },
})
// resp.Notification (in-app) and resp.Deliveries (per-medium email outcome)
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

Set a notification preference for a recipient. Pass an optional `Medium`
(`bodhveda.MediumInApp` or `bodhveda.MediumEmail`) to toggle in-app and email
independently for the same target. Medium defaults to `in_app` when omitted.

```go
resp, _ := client.Recipients.Preferences.Set(ctx, "user-123", &bodhveda.SetPreferenceRequest{
    Target: bodhveda.Target{
        Channel: "digest",
        Topic:   "none",
        Event:   "sent",
    },
    Medium: bodhveda.MediumEmail,
    State:  struct{ Enabled bool `json:"enabled"` }{Enabled: true},
})
// resp.Target, resp.State
```

### Check a preference

Check the state of a specific preference for a recipient.

```go
resp, _ := client.Recipients.Preferences.Check(ctx, "user-123", &bodhveda.CheckPreferenceRequest{
    Target: bodhveda.Target{
        Channel: "digest",
        Topic:   "none",
        Event:   "sent",
    },
    Medium: bodhveda.MediumEmail,
})
// resp.Target, resp.State
```

---

## Recipient Contacts

Contacts are per-medium addresses for a recipient. To send **email** to a recipient,
add an `email` contact and mark it primary. Sync this **server-side** (e.g. on your
`/me` endpoint) so the address never rides a browser request.

`Create`, `List`, and `Update` work with a `Full access` or `Recipient access` API key;
`Delete` requires `Full access`.

### Add a contact

```go
resp, _ := client.Recipients.Contacts.Create(ctx, "user-123", &bodhveda.CreateRecipientContactRequest{
    Medium:    bodhveda.MediumEmail,
    Address:   "alice@example.com",
    IsPrimary: true,
})
// resp.RecipientContact
```

### List contacts

```go
resp, _ := client.Recipients.Contacts.List(ctx, "user-123")
// resp.Contacts
```

### Update a contact

```go
newAddress := "alice.new@example.com"
resp, _ := client.Recipients.Contacts.Update(ctx, "user-123", 1, &bodhveda.UpdateRecipientContactRequest{
    Address: &newAddress,
})
// resp.RecipientContact
```

### Delete a contact

Requires a `Full access` API key.

```go
err := client.Recipients.Contacts.Delete(ctx, "user-123", 1)
```

---

## License

MIT
