import { PaginationMeta } from "@/lib/types";

export type NotificationKind = "direct" | "broadcast";

// The list endpoint also accepts "all" (both kinds), which the recipient detail
// feed uses. The kind TOGGLE deliberately still offers only direct|broadcast —
// those tables have different columns and cannot be merged.
export type NotificationKindFilter = NotificationKind | "all";

export type NotificationStatus =
    | "enqueued"
    | "muted"
    | "delivered"
    | "quota_exceeded"
    | "failed";

export type BroadcastStatus =
    | "enqueued"
    | "completed"
    | "quota_exceeded"
    | "failed";

// Per-(notification, medium) delivery status. Email is the only non-in_app
// medium written today. `pending → sending → sent` are set by the worker;
// `delivered → bounced → complained` arrive via provider webhooks.
export type DeliveryStatus =
    | "pending"
    | "sending"
    | "sent"
    | "delivered"
    | "bounced"
    | "complained"
    | "failed"
    | "muted"
    | "no_contact"
    | "suppressed"
    | "quota_exceeded"
    | "rejected";

// The email-medium delivery summary on a listed notification. Carries every
// BOUNDED delivery column, so the list can explain an outcome inline and the
// detail dialog can render the lifecycle without waiting on a fetch.
//
// The raw webhook event history (provider_response) is deliberately absent — it
// is unbounded, so it is fetched per-notification via useNotificationDeliveries.
export interface NotificationEmailDelivery {
    status: DeliveryStatus;
    // The only thing separating the two causes of `muted`: `not_cataloged` vs
    // `preference_disabled`. See deliveryFailureReasonText().
    failure_reason?: string;
    attempt: number;
    provider?: string;
    provider_message_id?: string;
    address_snapshot?: string;
    sent_at?: string;
    delivered_at?: string;
    bounced_at?: string;
    complained_at?: string;
    // Soft, directional signals only (Apple MPP inflates opens) — never present
    // these as equivalent to in-app `read`.
    opened_at?: string;
    clicked_at?: string;
}

// One entry of a delivery's provider_response array: a raw provider webhook body
// (appended once per webhook), reduced to what a timeline needs. `kind` and `at`
// are normalized SERVER-side by the provider's adapter, so the console never
// parses a provider's JSON shape. `kind` is empty for an unrecognized event.
export interface DeliveryEvent {
    kind: string;
    at?: string;
    raw: unknown;
}

// The full delivery record for one (notification, medium), including the webhook
// event history. Served per-notification, not on list rows.
export interface NotificationDeliveryDetail {
    id: number;
    medium: string;
    status: DeliveryStatus;
    failure_reason?: string;
    attempt: number;
    provider?: string;
    provider_message_id?: string;
    // The address captured at enqueue — immune to later contact edits, so it
    // reflects where this email actually went.
    address_snapshot?: string;
    sent_at?: string;
    delivered_at?: string;
    bounced_at?: string;
    complained_at?: string;
    opened_at?: string;
    clicked_at?: string;
    events: DeliveryEvent[];
    created_at: string;
    updated_at: string;
}

export interface ListNotificationDeliveriesResult {
    deliveries: NotificationDeliveryDetail[];
}

export interface Notification {
    id: number;
    recipient_id: string;
    // The in-app content block, as sent. The API serializes it from
    // json.RawMessage, so this is arbitrary customer JSON (an object in
    // practice), NOT a string.
    payload: unknown;
    broadcast_id: number | null;
    target: Target;
    state: NotificationState;
    status: NotificationStatus;
    completed_at?: string;
    created_at: string;
    updated_at: string;
    // Present only when the send included an email block. Lets the list show
    // the email outcome beside the in-app status.
    email?: NotificationEmailDelivery;
}

export interface Broadcast {
    id: number;
    payload: string;
    target: Target;
    status: BroadcastStatus;
    completed_at?: string;
    created_at: string;
    updated_at: string;
}

export interface Target {
    channel: string;
    topic: string;
    event: string;
}

interface NotificationState {
    read: boolean;
    opened: boolean;
}

export interface EmailContent {
    subject: string;
    html?: string;
    text?: string;
}

export interface SendNotificationPayload {
    recipient_id: string | null;
    target: Target | null;
    payload: unknown;
    // Optional typed email block. Present ⇒ email is attempted (direct sends
    // only); absent ⇒ no email. Gated by catalog + per-medium preference + a
    // primary email contact.
    email?: EmailContent;
}

export interface NotificationDelivery {
    medium: string;
    status: string;
    address?: string;
    failure_reason?: string;
    created_at: string;
    updated_at: string;
}

export interface NotificationsOverviewResult {
    total_notifications: number;
    total_direct_sent: number;
    total_broadcast_sent: number;
}

export interface SendNotificationResult {
    notification: Notification | null;
    broadcast: Broadcast | null;
    // Per-medium delivery outcomes for a direct send (email). A partial-medium
    // failure never rejects the send — the outcome is reported here.
    deliveries?: NotificationDelivery[];
}

export interface ListNotificationsPayload {
    kind: NotificationKindFilter;
    page?: number;
    limit?: number;
    /** Exact recipient external id. Omit for the whole project. */
    recipient_id?: string;
}

export interface ListNotificationsResult {
    notifications: Notification[];
    pagination: PaginationMeta;
}

export interface BroadcastListItem extends Broadcast {
    delivered_count: number;
    read_count: number;
    opened_count: number;
}

export interface ListBroadcastsPayload {
    page?: number;
    limit?: number;
}

export interface ListBroadcastsResult {
    broadcasts: BroadcastListItem[];
    pagination: PaginationMeta;
}

// EmailDeliveryOverview aggregates the project's email delivery rows into
// per-status counts (Phase 5). `opened` / `clicked` come from provider webhooks
// and are directional soft signals — email "opened" is unreliable (Apple Mail
// Privacy Protection inflates it), unlike in-app "read".
export interface EmailDeliveryOverview {
    total: number;
    pending: number;
    sent: number;
    delivered: number;
    bounced: number;
    complained: number;
    failed: number;
    no_contact: number;
    muted: number;
    opened: number;
    clicked: number;
}
