import { PaginationMeta } from "@/lib/types";

export type NotificationKind = "direct" | "broadcast";

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

export interface NotificationEmailDelivery {
    status: DeliveryStatus;
    sent_at?: string;
    delivered_at?: string;
}

export interface Notification {
    id: number;
    recipient_id: string;
    payload: string;
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
    kind: NotificationKind;
    page?: number;
    limit?: number;
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
