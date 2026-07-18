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
    -   [Recipient Contacts](#recipient-contacts)
-   [Project Preferences](#project-preferences)
-   [License](#license)

## Installation

```bash
npm install @bodhveda/js
```

> Previously published as `bodhveda`. That package is deprecated — install `@bodhveda/js` instead.

## Quick Start

```typescript
import { Bodhveda } from "@bodhveda/js";

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

### Send with email

Include the optional `email` block to also send an email. Its presence makes email
eligible (**direct sends only** — an email block on a broadcast returns `400`).
Bodhveda does no templating: you render the subject/HTML/text yourself (e.g. with
`@react-email`) and pass the result. `text` is optional and derived from `html`.

Email fires only when the `(target, email)` pair is cataloged, the recipient's email
preference is enabled, and the recipient has a primary email
[contact](#recipient-contacts). Per-medium outcomes are returned in `deliveries`.

```typescript
const res = await bodhveda.notifications.send({
    recipient_id: "user-123",
    target: { channel: "digest", topic: "none", event: "sent" },
    payload: { title: "Your daily digest is ready." },
    email: {
        subject: "Your daily digest",
        html: "<h1>Your daily digest</h1><p>3 new follow-ups today.</p>",
    },
});
// res.notification (in-app) and res.deliveries (per-medium email outcome)
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

Set a notification preference for a recipient. Pass an optional `medium`
(`"in_app"` or `"email"`) to toggle in-app and email independently for the same
target. It defaults to `"in_app"` when omitted.

```typescript
await bodhveda.recipients.preferences.set("user-123", {
    target: { channel: "digest", topic: "none", event: "sent" },
    medium: "email",
    state: { enabled: true },
});
```

### Check a preference

Check the state of a specific preference for a recipient.

```typescript
const result = await bodhveda.recipients.preferences.check("user-123", {
    target: { channel: "digest", topic: "none", event: "sent" },
    medium: "email",
});
```

---

## Recipient Contacts

Contacts are per-medium addresses for a recipient. To send **email** to a recipient,
add an `email` contact and mark it primary. Sync this **server-side** (e.g. on your
`/me` endpoint) so the address never rides a browser request.

`create`, `list`, and `update` work with a `Full access` or `Recipient access` API
key; `delete` requires `Full access`.

### Add a contact

```typescript
await bodhveda.recipients.contacts.create("user-123", {
    medium: "email",
    address: "alice@example.com",
    is_primary: true,
});
```

`create` is strict — it rejects with a `409` when the contact already exists. To
keep a recipient's primary address current from a server without a
list-then-diff, use `setPrimary` instead.

### Set the primary contact

Idempotently ensure an address is the recipient's **primary** contact for a
medium — create it if absent, update the existing primary in place if the
address differs (which resets verification), or no-op if it already matches.
Returns the resulting primary contact (`200`) in every case, so a "keep the
primary email current" sync is a single call.

```typescript
await bodhveda.recipients.contacts.setPrimary("user-123", {
    medium: "email",
    address: "alice@example.com",
});
```

### List contacts

```typescript
const { contacts } = await bodhveda.recipients.contacts.list("user-123");
```

### Update a contact

```typescript
await bodhveda.recipients.contacts.update("user-123", 1, {
    address: "alice.new@example.com",
});
```

### Delete a contact

Requires a `Full access` API key.

```typescript
await bodhveda.recipients.contacts.delete("user-123", 1);
```

---

## Project Preferences

The **project preference catalog** declares which `(target, medium)` pairs your
project may send, and the default a recipient inherits until they set a
preference of their own. This is different from
[Recipient Preferences](#recipient-preferences), which are a single recipient's
own toggles — manage the catalog with `bodhveda.preferences`.

Requires a `Full access` API key, and is a **server-side** concern. The project
is taken from the API key.

### List the catalog

```typescript
const catalog = await bodhveda.preferences.list();
```

### Create a catalog entry

Strict — rejects with a `409` when an entry for the same
`(channel, topic, event, medium)` already exists. `medium` defaults to `in_app`
when omitted.

```typescript
const entry = await bodhveda.preferences.create({
    channel: "digest",
    topic: "none",
    event: "sent",
    medium: "email",
    label: "Daily digest",
    default_enabled: true,
});
```

### Get / update / delete an entry

The natural key (`channel`/`topic`/`event`/`medium`) is immutable — `update`
changes only the label and default.

```typescript
const entry = await bodhveda.preferences.get(123);

await bodhveda.preferences.update(123, {
    label: "Weekly digest",
    default_enabled: false,
});

// Un-catalogs the (target, medium).
await bodhveda.preferences.delete(123);
```

### Set up a whole catalog at once

`upsertMany` declaratively merges an entire catalog in one call — the primitive
for a one-off "set up my project's preferences" script. Each item is upserted by
its natural key (new inserted, existing label + default updated). Entries **not**
in the array are left untouched.

```typescript
await bodhveda.preferences.upsertMany([
    { channel: "digest", topic: "none", event: "sent", medium: "email", label: "Daily digest", default_enabled: true },
    { channel: "posts", topic: "any", event: "new_comment", medium: "email", label: "Comments", default_enabled: true },
]);
```

Pass `{ prune: true }` to also **delete** entries absent from the array, making
it the entire desired catalog. Pruning un-catalogs a `(target, medium)`, which
turns a non-`in_app` medium off for recipients relying on the catalog default —
so it is opt-in.

```typescript
await bodhveda.preferences.upsertMany(desiredCatalog, { prune: true });
```

---

## License

MIT
