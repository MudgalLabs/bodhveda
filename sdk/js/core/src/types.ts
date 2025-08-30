/**
 * Represents a target for notifications.
 */
export interface Target {
    channel: string;
    topic: string;
    event: string;
}

/**
 * Represents a preference target, extending the Target interface.
 */
export interface TargetWithLabel extends Target {
    label?: string;
}

/**
 * Represents the state of a preference.
 */
export interface PreferenceState {
    enabled: boolean;
    inherited: boolean;
}

/**
 * Represents a preference with a target and state.
 */
export interface Preference {
    target: TargetWithLabel;
    state: PreferenceState;
}

/**
 * Represents the state of a notification.
 */
export interface NotificationState {
    opened: boolean;
    read: boolean;
}

/**
 * Represents a notification.
 */
export interface Notification {
    id: number;
    recipient_id: string;
    payload: unknown;
    target: Target;
    state: NotificationState;
    broadcast_id: number | null;
    created_at: string;
    updated_at: string;
}

/**
 * Represents a broadcast.
 */
export interface Broadcast {
    id: number;
    payload: unknown;
    target: Target;
    created_at: string;
    updated_at: string;
}

/**
 * Represents a recipient.
 */
export interface Recipient {
    id: string;
    name: string;
    created_at: string;
    updated_at: string;
}

/**
 * Represents a request to create a recipient.
 */
export interface CreateRecipientRequest {
    id: string;
    name?: string;
}

/**
 * Represents the response after creating a recipient.
 */
export interface CreateRecipientResponse extends Recipient {}

/**
 * Represents a request to create multiple recipients in a batch.
 */
export interface CreateRecipientsBatchRequest {
    recipients: CreateRecipientRequest[];
}

/**
 * Represents a result item for batch creation of recipients.
 */
interface BatchCreateRecipientResultItem {
    id: string;
}

/**
 * Represents a result item with error details for batch creation of recipients.
 */
interface BatchCreatereRecicpientResultItemWithError
    extends BatchCreateRecipientResultItem {
    batch_index: number;
    errors: {
        message: string;
        description: string;
        property_path?: string;
        invalid_value?: unknown;
    }[];
}

/**
 * Represents the response after creating multiple recipients in a batch.
 */
export interface CreateRecipientsBatchResponse {
    created: BatchCreateRecipientResultItem[];
    updated: BatchCreateRecipientResultItem[];
    failed: BatchCreatereRecicpientResultItemWithError[];
}

/**
 * Represents the response after retrieving a recipient.
 */
export interface GetRecipientResponse extends Recipient {}

/**
 * Represents a request to update a recipient.
 */
export interface UpdateRecipientRequest {
    name?: string;
}

/**
 * Represents the response after updating a recipient.
 */
export interface UpdateRecipientResponse extends Recipient {}

/**
 * Represents a request to send a notification.
 */
export interface SendNotificationRequest {
    payload: unknown;
    recipient_id?: string;
    target?: Target;
}

/**
 * Represents the response after sending a notification.
 */
export interface SendNotificationResponse {
    notification: Notification | null;
    broadcast: Broadcast | null;
}

/**
 * Represents a request to list notifications.
 */
export interface ListNotificationsRequest {
    limit?: number;
    before?: string;
    after?: string;
}

/**
 * Represents the response after listing notifications.
 */
export interface ListNotificationsResponse {
    notifications: Notification[];
    cursor: {
        before: string | null;
        after: string | null;
    };
}

/**
 * Represents the response with the count of unread notifications.
 */
export interface UnreadCountResponse {
    unread_count: number;
}

/**
 * Represents a request to update the state of notifications.
 */
export interface UpdateNotificationsStateRequest {
    ids: number[];
    state: Partial<NotificationState>;
}

/**
 * Represents the response after updating the state of notifications.
 */
export interface UpdateNotificationsStateResponse {
    updated_count: number;
}

/**
 * Represents a request to delete notifications.
 */
export interface DeleteNotificationsRequest {
    ids: number[];
}

/**
 * Represents the response after deleting notifications.
 */
export interface DeleteNotificationsResponse {
    deleted_count: number;
}

/**
 * Represents the response after listing preferences.
 */
export interface ListPreferencesResponse {
    preferences: Preference[];
}

/**
 * Represents a request to set a preference.
 */
export interface SetPreferenceRequest {
    target: Target;
    state: {
        enabled: boolean;
    };
}

/**
 * Represents the response after setting a preference.
 */
export interface SetPreferenceResponse {
    target: Target;
    state: PreferenceState;
}

/**
 * Represents a request to check a preference.
 */
export interface CheckPreferenceRequest {
    target: Target;
}

/**
 * Represents the response after checking a preference.
 */
export interface CheckPreferenceResponse {
    target: Target;
    state: PreferenceState;
}
