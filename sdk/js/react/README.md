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

## Building a settings screen

The preference hooks are the **recipient** surface: they read and write one
person's opt-ins with a recipient-scoped key. The **catalog** — the project-level
list of `(target, medium)` pairs that may fire — is a server-side concern with a
full-access key, so it is a `@bodhveda/js` job, not a hook. Two things it decides
still show up here.

**`mandatory` — render a lock, not a switch.** Each resolved preference carries
`state.mandatory`. A mandatory entry is one recipients may not opt out of
(password resets, security alerts, a one-shot welcome): it outranks any rule they
already had, and `useUpdatePreference` on it is refused with a `400`. So it has to
be rendered as locked — a toggle that saves and changes nothing is worse than one
that refuses.

```tsx
const { data } = usePreferences();
const { mutate: update } = useUpdatePreference();

return data?.preferences.map(({ target, state }) => {
    const { channel, topic, event, medium, name } = target;

    return (
        <Row key={`${channel}/${topic}/${event}/${medium}`}>
            {/* `name` is the catalog entry's label, so it is set only for a
                cataloged target — see below. */}
            <span>{name ?? `${channel}/${topic}/${event}`}</span>
            {state.mandatory ? (
                <LockIcon aria-label="Always on" />
            ) : (
                <Switch
                    checked={state.enabled}
                    onChange={(enabled) =>
                        update({
                            target: { channel, topic, event },
                            medium,
                            state: { enabled },
                        })
                    }
                />
            )}
        </Row>
    );
});
```

**`cataloged` — why a target is missing from this screen.** `usePreferences()`
returns the catalog resolved against the recipient, so a target nobody cataloged
appears on no settings screen and no recipient can mute it. If an event is
missing here, the fix is in your deploy pipeline (seed the catalog with
`bodhveda.preferences.upsertMany` from `@bodhveda/js`), not in this component.

**Strict targets** is a per-project setting, **off by default**, that turns the
catalog into a gate: with it on, *sending* to an uncataloged `(target, medium)` is
a `400`. It never changes what these hooks do — they read and write preferences,
they do not send — but it is why a well-seeded catalog matters, and it is the
setting that turns a missing entry from a silent no-op into a loud failure on your
server. See the [`@bodhveda/js` README](https://www.npmjs.com/package/@bodhveda/js)
for the send-side details.

## License

MIT
