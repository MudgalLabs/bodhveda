# JavaScript/TypeScript SDK for Bodhveda

Official JavaScript/TypeScript SDK for Bodhveda.

It offers a simpler way to work with Bodhveda APIs in both browser and server environments.

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
npm install bodhveda
```

## Quick Start

```typescript
import { Bodhveda } from "bodhveda";

const bodhveda = new Bodhveda("YOUR_API_KEY");

// Send a notification to a recipient.
// Note: Bodhveda will create the recipient if it does not already exist.
await bodhveda.notifications.send({
    recipient_id: "user-123",
    payload: { message: "Hello, world!" },
});

// List all notifications for a recipient.
const notifications = await bodhveda.recipients.notifications.list("user-123");
```

## Notifications

### Send a notification

Send a notification to a recipient or broadcast to a target.

```typescript
await bodhveda.notifications.send({
    recipient_id: "user-123",
    payload: { message: "Hello, world!" },
});
```

---

## Recipients

### Create a recipient

Create a new recipient.

```typescript
await bodhveda.recipients.create({
    id: "user-123",
    name: "Alice",
});
```

### Create multiple recipients (batch)

Create multiple recipients in a single request.

```typescript
await bodhveda.recipients.createBatch({
    recipients: [
        { id: "user-1", name: "Alice" },
        { id: "user-2", name: "Bob" },
    ],
});
```

### Get a recipient

Retrieve details of a recipient by ID.

```typescript
const recipient = await bodhveda.recipients.get("user-123");
```

### Update a recipient

Update recipient details.

```typescript
await bodhveda.recipients.update("user-123", { name: "Alice Updated" });
```

### Delete a recipient

Delete a recipient by ID.

```typescript
await bodhveda.recipients.delete("user-123");
```

---

## Recipient Notifications

### List notifications

List notifications for a recipient.

```typescript
const notifications = await bodhveda.recipients.notifications.list("user-123");
```

### Get unread notification count

Get the count of unread notifications for a recipient.

```typescript
const { unread_count } = await bodhveda.recipients.notifications.unreadCount(
    "user-123"
);
```

### Update notification state

Update the state (e.g., mark as read) of notifications for a recipient.

```typescript
await bodhveda.recipients.notifications.updateState("user-123", {
    ids: [1, 2, 3],
    state: { read: true },
});
```

### Delete notifications

Delete notifications for a recipient.

```typescript
await bodhveda.recipients.notifications.delete("user-123", {
    ids: [1, 2, 3],
});
```

---

## Recipient Preferences

### List preferences

List all preferences for a recipient.

```typescript
const preferences = await bodhveda.recipients.preferences.list("user-123");
```

### Set a preference

Set a notification preference for a recipient.

```typescript
await bodhveda.recipients.preferences.set("user-123", {
    target: { channel: "email", topic: "news", event: "daily" },
    state: { enabled: true },
});
```

### Check a preference

Check the state of a specific preference for a recipient.

```typescript
const result = await bodhveda.recipients.preferences.check("user-123", {
    target: { channel: "email", topic: "news", event: "daily" },
});
```

---

## License

MIT
