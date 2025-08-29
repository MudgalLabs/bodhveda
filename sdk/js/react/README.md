# React SDK for Bodhveda

Official React SDK for Bodhveda.

It extends the core `bodhveda` SDK to provide you with hooks to make it easy to build custom notification UX with React.

> This SDK uses [TanStack Query](https://tanstack.com/query/v5/docs/framework/react/overview) to manage Bodhveda API state for you - including caching and invalidation as well. You will need to add `@tanstack/react-query` and wrap your React app with `QueryClientProvider` and then put `BodhvedaProvider` inside it so that `@bodhveda/react` can use the ReactQuery's `QueryClient`.

## Installation

```bash
npm install @bodhveda/react @tanstack/react-query
```

## Usage

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BodhvedaProvider } from "@bodhveda/react";

const queryClient = new QueryClient();

<QueryClientProvider client={queryClient}>
    <BodhvedaProvider apiKey="your-api-key" recipientID="user-123">
        <NotificationInbox />
    </BodhvedaProvider>
</QueryClientProvider>;

// src/components/NotificationInbox.tsx
import { Notification } from "./Notification";
import { useNotifications } from "@bodhveda/react";

function NotificationInbox() {
    // Fetch recipient's notifications.
    const { data } = useNotifications();

    // ...
    // Handle loading and error states.
    // ...

    // Render the notifications however you want.
    return (
        <ul>
            {data.notifications.map((notification) => (
                <li key={notification.id}>
                    <Notification notification={notification} />
                </li>
            ))}
        </ul>
    );
}
```

## Hooks

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
