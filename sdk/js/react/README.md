# @bodhveda/react

Official React SDK for Bodhveda.

It extends the core `bodhveda` SDK with React hooks making it easy to build custom notification experiences.

## Installation

```bash
npm install @bodhveda/react @tanstack/react-query
```

## Usage

Wrap your app with the `BodhvedaProvider`:

```tsx
import { BodhvedaProvider } from "@bodhveda/react";

<BodhvedaProvider apiKey="your-api-key" recipientID="user-123">
    <App />
</BodhvedaProvider>;
```

Or provide an existing Bodhveda client:

```tsx
import { Bodhveda } from "bodhveda";
import { BodhvedaProvider } from "@bodhveda/react";

const client = new Bodhveda("your-api-key");

<BodhvedaProvider bodhveda={client} recipientID="user-123">
    <App />
</BodhvedaProvider>;
```

## Hooks

All hooks must be used within a `BodhvedaProvider`.

### `useBodhveda()`

Returns the Bodhveda client instance.

### `useRecipientID()`

Returns the current recipient ID.

### `useNotifications(options?)`

Fetches the list of notifications for the current recipient.

### `useNotificationsUnreadCount(options?)`

Fetches the unread notifications count for the current recipient.

### `useUpdateNotificationsState(options?)`

Returns a mutation hook to update notification state (e.g., mark as read).

### `useDeleteNotifications(options?)`

Returns a mutation hook to delete notifications for the current recipient.

### `usePreferences(options?)`

Fetches the notification preferences for the current recipient.

### `useUpdatePreference(options?)`

Returns a mutation hook to update a notification preference.

### `useCheckPreference(target, options?)`

Checks a specific notification preference for the current recipient.

## License

MIT
