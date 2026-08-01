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
-   [Project Preferences](#project-preferences)
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
[contact](#recipient-contacts).

> [!IMPORTANT]
> **You can only send a target you have cataloged.** The catalog is a gate: if no
> project preference exists for the `(target, email)` pair, the send is rejected with
> a `400` and nothing is written. The gate applies per medium to the mediums a send
> actually asks for — a send carrying only `Payload` needs an `in_app` entry, one
> carrying `Email` needs an `email` entry, one carrying both needs both.
>
> A `topic: any` catalog entry satisfies the gate for every concrete topic beneath it,
> so one entry covers an unbounded set of runtime-generated targets.
>
> Catalog the pair with [`UpsertMany`](#set-up-a-whole-catalog-at-once), and run that
> **before the new code starts serving**, on every deploy — otherwise code that sends a
> target you haven't cataloged yet 400s on every notification.
>
> Once through the gate, email still needs the recipient's email preference enabled and
> a primary email contact. Those resolve asynchronously, so read the outcome back
> rather than inferring it from the send:
>
> ```go
> n, _ := client.Notifications.Get(ctx, resp.Notification.ID)
> fmt.Println(n.Email.Status, n.Email.FailureReason)
> ```

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
// resp.Notification.ID — the send is accepted (Status "enqueued"); the email is
// resolved asynchronously. Read the outcome back with Notifications.Get below.
```

### Get a notification (check the outcome)

The send is **asynchronous**: it accepts the notification and returns its id
(`Status: "enqueued"`), then the worker resolves in-app delivery and the email.
Fetch the notification by id to see the resolved in-app `Status` and, when the send
included an `Email` block, the email delivery outcome on `Email`.

```go
n, _ := client.Notifications.Get(ctx, resp.Notification.ID)

n.Status // "delivered" | "muted" | "quota_exceeded" | "failed" | "enqueued"
if n.Email != nil {
    n.Email.Status      // "pending" | "sent" | "delivered" | "bounced" | ...
    n.Email.DeliveredAt // *time.Time
}
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

### Set the primary contact (idempotent)

Use this for a server-side sync that keeps a recipient's primary email current.
Unlike `Create`, it does not fail (409) when the contact already exists — it
creates the primary if absent, updates the existing primary's address if it
differs (which resets verification), or no-ops if it already matches, all in a
single call.

```go
resp, _ := client.Recipients.Contacts.SetPrimary(ctx, "user-123", &bodhveda.SetPrimaryContactRequest{
    Medium:  bodhveda.MediumEmail,
    Address: "alice@example.com",
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

## Project Preferences

The project preference **catalog** declares which `(target, medium)` pairs your
project may send and the default a recipient inherits until they set a toggle of
their own. This is `client.Preferences` — project-scoped by the API key. It is
distinct from `client.Recipients.Preferences`, which manages a single recipient's
own toggles.

All catalog methods require a `Full access` API key — the catalog defines what a
whole project may send, so it has no place on a recipient-scoped key.

> [!IMPORTANT]
> **The catalog is a gate, so seeding it is a deploy step, not a one-time setup task.**
>
> A `(target, medium)` that isn't cataloged can't be sent at all — the send 400s and
> writes nothing. Your desired catalog lives in your code and the real one lives in
> Bodhveda, so the two drift the moment you ship an event and forget to update
> Bodhveda to match.
>
> Derive the slice from whatever already defines your events, and run
> [`UpsertMany`](#set-up-a-whole-catalog-at-once) from your deploy pipeline on **every**
> deploy. It is an idempotent merge, so a deploy that changes nothing is a no-op.
>
> Seed **before** the new code starts serving, and let a failed seed **fail the
> deploy** — stopping leaves the previous version running, which only sends targets
> that are already cataloged.
>
> Because the desired catalog lives in your code and the real one lives in Bodhveda,
> the two drift the moment you ship an event and forget to re-run the seed. Keep them
> honest by deriving the slice from your app's event definitions and running
> [`UpsertMany`](#set-up-a-whole-catalog-at-once) from your deploy pipeline. It is an
> idempotent merge, so running it on every deploy costs nothing when nothing changed.

### List the catalog

```go
prefs, _ := client.Preferences.List(ctx)
```

### Create a catalog entry

Strict — rejects with a 409 when an entry for the same `(channel, topic, event,
medium)` already exists. `Medium` defaults to `in_app` when empty.

```go
pref, _ := client.Preferences.Create(ctx, &bodhveda.CreateProjectPreferenceRequest{
    Channel:        "posts",
    Topic:          "any",
    Event:          "new_comment",
    Medium:         bodhveda.MediumEmail,
    Name:           "New comments",
    Description:    "Get notified when someone comments on your posts.",
    DefaultEnabled: true,
})
```

### Get a catalog entry

```go
pref, _ := client.Preferences.Get(ctx, pref.ID)
```

### Update a catalog entry

The natural key (`channel`/`topic`/`event`/`medium`) is immutable, so only the
name, description and default change.

```go
pref, _ := client.Preferences.Update(ctx, pref.ID, &bodhveda.UpdateProjectPreferenceRequest{
    Name:           "New comments on your posts",
    Description:    "Get notified when someone comments on your posts.",
    DefaultEnabled: false,
})
```

### Delete a catalog entry

```go
err := client.Preferences.Delete(ctx, pref.ID)
```

### Set up a whole catalog in one call

`UpsertMany` declaratively merges a whole catalog — the primitive for a one-off
"set up my project's preferences" script. Each item is upserted by its natural
key. By default entries absent from the slice are left untouched; pass
`&bodhveda.UpsertProjectPreferencesOptions{Prune: true}` to also delete them,
making the slice the entire desired catalog.

```go
prefs, _ := client.Preferences.UpsertMany(ctx, []bodhveda.UpsertProjectPreferenceItem{
    {Channel: "posts", Topic: "any", Event: "new_comment", Medium: bodhveda.MediumInApp, Name: "New comments", DefaultEnabled: true},
    {Channel: "posts", Topic: "any", Event: "new_comment", Medium: bodhveda.MediumEmail, Name: "New comments", DefaultEnabled: false},
}, &bodhveda.UpsertProjectPreferencesOptions{Prune: true})
```

---

## License

MIT
