import { PaginationMeta } from "@/lib/types";

export type NotificationKind = "direct" | "broadcast";

export interface Notification {
    id: number;
    recipient_id: string;
    payload: string;
    broadcast_id: number | null;
    target: Target;
    state: NotificationState;
    created_at: string;
    updated_at: string;
}

export interface Broadcast {
    id: number;
    payload: string;
    target: Target;
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

export interface SendNotificationPayload {
    recipient_id: string | null;
    target: Target | null;
    payload: unknown;
}

export interface NotificationsOverviewResult {
    total_notifications: number;
    total_direct_sent: number;
    total_broadcast_sent: number;
}

export interface SendNotificationResponse {
    notification: Notification | null;
    broadcast: Broadcast | null;
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
