import { PaginationMeta } from "@/lib/types";

export const NOTIFICATION_KINDS = ["direct", "broadcast"] as const;

export type NotificationKind = (typeof NOTIFICATION_KINDS)[number];

export const DEFAULT_NOTIFICATION_KIND: NotificationKind = "direct";

// The list endpoint also accepts "all" (both kinds), which the recipient detail
// feed uses. The kind TOGGLE deliberately still offers only direct|broadcast —
// those tables have different columns and cannot be merged.
export type NotificationKindFilter = NotificationKind | "all";

export const NOTIFICATION_STATUSES = [
    "enqueued",
    "muted",
    "delivered",
    "quota_exceeded",
    "failed",
    // An email-only send: the caller passed an `email` block but no `payload`, so
    // no in-app row was ever requested. Set at INSERT and never resolved, unlike
    // every other status here. The notification row exists only to carry the
    // email delivery, the analytics join, and GET /notifications/{id} — it is
    // hidden from the recipient's own feed and unread count, but deliberately
    // VISIBLE in the console, which is the operator's view.
    "not_requested",
] as const;

export type NotificationStatus = (typeof NOTIFICATION_STATUSES)[number];

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

// The delivery statuses an email can actually reach in v1. The API validates
// against the full notification_delivery CHECK (12 values), but four of those
// — sending / suppressed / quota_exceeded / rejected — are reserved and never
// written, so offering them as filters would imply data that cannot exist.
// The console offers what can occur; the API keeps accepting what is legal.
export const EMAIL_DELIVERY_FILTER_STATUSES = [
    "pending",
    "sent",
    "delivered",
    "bounced",
    "complained",
    "failed",
    "muted",
    "no_contact",
] as const;

// The email filter folds the medium and delivery-status dimensions into one
// control, because in v1 they are not independent: `email` is the only medium
// with a delivery row at all (in_app keeps its outcome on the notification row,
// filtered by `status`). So there are only three questions to ask about email —
// was it skipped entirely (`none`), was it attempted (`any`), or how did it end
// (a status). See enum.EmailDeliveryFilter in the API for the same reasoning.
//
// `none` is the one that matters most: it is how in-app-only notifications —
// still the common case — stay findable rather than merely un-dropped.
export const EMAIL_FILTERS = [
    "none",
    "any",
    ...EMAIL_DELIVERY_FILTER_STATUSES,
] as const;

export type EmailFilter = (typeof EMAIL_FILTERS)[number];

/**
 * The operator's filter selection for the notifications list. Every field lives
 * in the URL, so a filtered view is shareable and survives a reload.
 *
 * `from` / `to` are `YYYY-MM-DD` calendar days, NOT instants — see
 * notificationFiltersToParams for where they become an absolute range.
 */
export interface NotificationFilters {
    kind: NotificationKind;
    status?: NotificationStatus;
    email?: EmailFilter;
    channel?: string;
    topic?: string;
    event?: string;
    recipient_search?: string;
    from?: string;
    to?: string;
}

export interface ListNotificationsPayload {
    kind: NotificationKindFilter;
    page?: number;
    limit?: number;
    /** Exact recipient external id. Omit for the whole project. */
    recipient_id?: string;
    status?: NotificationStatus;
    email?: EmailFilter;
    channel?: string;
    topic?: string;
    event?: string;
    recipient_search?: string;
    /** Absolute RFC3339 instants, derived from the picked days. */
    created_from?: string;
    created_to?: string;
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

// Outcome is the coarse, medium-independent answer to "how did this end up?",
// mirroring enum.Outcome server-side.
//
// ⚠️ `suppressed` is NOT a failure. It means the recipient opted out or has no
// contact address — the system working as designed. Rendering it as an error
// makes a healthy project look broken and makes every failure count wrong.
//
// `not_requested` is in-app only: the SENDER never asked for this medium (an
// email-only send). Not a success, not a failure — the request restated.
export type DeliveryOutcome =
    | "pending"
    | "succeeded"
    | "suppressed"
    | "failed"
    | "not_requested";

// DeliveryTreeAudience is the recipient breakdown FROZEN when the broadcast
// fanned out. Absent on broadcasts sent before the counts existed, and on ones
// whose fan-out has not run — the UI must say "not recorded", never show 0.
export interface DeliveryTreeAudience {
    total: number;
    eligible: number;
    // The RECIPIENT opted out. Healthy; nothing to fix.
    excluded_disabled: number;
    // The PROJECT never offered this target for in-app (no catalog row, or a
    // disabled one). A config mistake, and the usual reason a broadcast reaches
    // nobody.
    excluded_not_cataloged: number;
    // ⚠️ false for broadcasts, and the UI must respect it: excluded recipients
    // are filtered out before any row is written, so there is nothing to drill
    // into. On a direct send the same case produces a real `muted` row you CAN
    // open. Rendering both identically would promise a drill-down that cannot
    // exist.
    expandable: boolean;
}

export interface DeliveryTreeMedium {
    medium: string;
    total: number;
    // Raw per-status counts, e.g. { delivered: 380, muted: 18 }.
    statuses: Record<string, number>;
    // The same data folded into DeliveryOutcome buckets. Both are carried: the
    // bucket colours the branch, the raw status explains it.
    outcomes: Record<string, number>;
    // Still in flight. Non-zero long after the send is the signature of a
    // stalled worker.
    pending: number;
}

export interface DeliveryTree {
    kind: "direct" | "broadcast";
    target: Target;
    audience?: DeliveryTreeAudience;
    mediums: DeliveryTreeMedium[];
}
