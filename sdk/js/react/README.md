# React SDK for Bodhveda

Official React SDK for Bodhveda.

It extends the core `@bodhveda/js` SDK to provide you with hooks to make it easy to build custom notification UX with React.

> This package re-exports everything from the core [`@bodhveda/js`](https://www.npmjs.com/package/@bodhveda/js) SDK, including the email/contacts types. The hooks here cover the recipient **inbox** (in-app notifications and preferences). Email delivery, recipient contacts, the project preference catalog, and provider configuration are server-side concerns — use the core SDK for those (email addresses and full-access keys should never ride a browser request).

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
    const { data, isLoading, isError, isFetching, fetchNextPage, hasNextPage } =
        useNotifications();

    // ...
    // Handle loading and error states.
    // ...

    // Render the notifications as per your requirements.
    return (
        <>
            <ul>
                {data.notifications.map((notification) => (
                    <li key={notification.id}>
                        <NotificationItem notification={notification} />
                    </li>
                ))}
            </ul>

            {hasNextPage && (
                <Button onClick={() => fetchNextPage()} loading={isFetching}>
                    Load more
                </Button>
            )}
        </>
    );
}
```

## Hooks

### `useBodhveda()`

Returns the Bodhveda client instance.

### `useRecipientID()`

Returns the current recipient ID.

### `useNotifications(req?, options?)`

Fetches notifications for the current recipient in a infinite scrolling manner.

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
