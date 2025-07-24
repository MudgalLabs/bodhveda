# Bodhveda Node.js SDK

The official Node.js SDK for [Bodhveda](https://bodhveda.com).

## Table of Contents

-   [Installation](#installation)
-   [Initialization](#initialization)
-   [Types](#types)
    -   [Payload](#payload)
    -   [Notification](#notification)
    -   [Broadcast](#broadcast)
-   [Error Handling](#error-handling)
-   [Notifications](#notifications)

    -   [send](#send)
    -   [fetch](#fetch)
    -   [unreadCount](#unreadcount)
    -   [markAsRead](#markasread)
    -   [markAllAsRead](#markallasread)
    -   [delete](#delete)
    -   [deleteAll](#deleteall)

-   [Broadcasts](#broadcasts)

    -   [send](#send-1)
    -   [fetch](#fetch-1)
    -   [delete](#delete-1)
    -   [deleteAll](#deleteall-1)

-   [License](#license)

---

## Installation

Install the SDK via npm:

```bash
npm install bodhveda
```

---

## Initialization

Import and initialize the SDK using your API key:

```ts
import { Bodhveda } from "bodhveda";

const bodhveda = new Bodhveda("YOUR_API_KEY");
```

> Replace `YOUR_API_KEY` with your actual Bodhveda API key. This key is secret. **Do not expose it in client-side code.**
>
> You can generate and manage your API keys from the **API Keys** tab in the Bodhveda's [dashboard](https://web.bodhveda.com/api-keys).

---

## Models

### Payload

```ts
type Payload =
    | string
    | number
    | boolean
    | Payload[]
    | { [key: string]: Payload };
```

### Notification

```ts
interface Notification {
    id: string;
    projectId: string;
    recipient: string;
    broadcastId?: string;
    payload: Payload;
    readAt?: string;
    createdAt: string;
    expiresAt: string;
}
```

### Broadcast

```ts
interface Broadcast {
    id: string;
    projectId: string;
    payload: Payload;
    createdAt: string;
    expiresAt: string;
}
```

---

## Error Handling

We recommend using `try/catch` blocks for robust error handling:

```ts
try {
    const notification = await bodhveda.notifications.send("recipient_123", {
        title: "Hello!",
        type: "info",
    });
} catch (error) {
    console.error("Failed to send notification:", error);
}
```

---

## Notifications

### send(recipient: string, payload: Payload): Promise\<Notification>

```ts
const notification = await bodhveda.notifications.send("recipient_123", {
    title: "Welcome!",
    type: "info",
    message: "Glad to have you onboard!",
    metadata: { plan: "pro" },
});
```

### fetch(recipient: string): Promise\<Notification\[]>

```ts
const notifications = await bodhveda.notifications.fetch("recipient_123");
```

### unreadCount(recipient: string): Promise<number>

```ts
const count = await bodhveda.notifications.unreadCount("recipient_123");
```

### markAsRead(recipient: string, notificationIds: string\[]): Promise\<void>

```ts
await bodhveda.notifications.markAsRead("recipient_123", [
    "notification_id_1",
    "notification_id_2",
]);
```

### markAllAsRead(recipient: string): Promise\<number>

```ts
const count = await bodhveda.notifications.markAllAsRead("recipient_123");
```

### delete(recipient: string, notificationIds: string\[]): Promise<number>

```ts
await bodhveda.notifications.delete("recipient_123", [
    "notification_id_1",
    "notification_id_2",
]);
```

### deleteAll(recipient: string): Promise\<number>

```ts
const count = await bodhveda.notifications.deleteAll("recipient_123");
```

---

## Broadcasts

### send(payload: BroadcastPayload): Promise\<Broadcast>

```ts
const broadcast = await bodhveda.broadcasts.send({
    system: true,
    message: "Server restart at midnight.",
});
```

### fetch(): Promise\<Broadcast\[]>

```ts
const broadcasts = await bodhveda.broadcasts.fetch();
```

### delete(broadcastIds: string\[]): Promise\<void>

```ts
await bodhveda.broadcasts.delete(["broadcast_id_1"]);
```

### deleteAll(): Promise\<number>

```ts
const count = await bodhveda.broadcasts.deleteAll();
```

---

## License

This SDK is licensed under the MIT License.

---

For more information, visit [bodhveda.com](https://bodhveda.com) or contact support.
