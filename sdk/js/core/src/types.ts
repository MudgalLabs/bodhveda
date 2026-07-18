/**
 * Represents a target for notifications.
 */
export interface Target {
    channel: string;
    topic: string;
    event: string;
}

/**
 * A medium a preference applies to. In-app and email are toggled independently
 * for the same target.
 */
export type PreferenceMedium = "in_app" | "email";

/**
 * Represents a preference target, extending the Target interface.
 */
export interface TargetWithLabel extends Target {
    /**
     * The medium this preference applies to (`in_app` or `email`).
     */
    medium?: PreferenceMedium;
    label?: string;
}

/**
 * Represents the state of a preference that was just written. It describes the
 * stored rule, so it carries no catalog context — reads answer a different
 * question and reply with {@link ResolvedPreferenceState}.
 */
export interface PreferenceState {
    enabled: boolean;
    /**
     * `true` when the recipient has no rule of their own for this exact
     * (target, medium).
     */
    inherited: boolean;
}

/**
 * What a preference read returns: whether a send would ACTUALLY deliver, plus
 * the context to explain it.
 */
export interface ResolvedPreferenceState {
    /**
     * The resolved decision — what a send to this (target, medium) would do.
     *
     * This is not a stored flag. It is resolved through the recipient's exact
     * rule, their `topic: "any"` rule, the project's exact rule, the project's
     * `topic: "any"` rule, and finally the medium's default — `in_app` delivers,
     * every other medium does not.
     */
    enabled: boolean;
    /**
     * `true` when the recipient has no rule of their own for this exact
     * (target, medium); the value came from elsewhere in the cascade.
     */
    inherited: boolean;
    /**
     * Whether a project-level rule exists for this exact (target, medium).
     *
     * Context for deciding what to render, **not** a gate: an explicit recipient
     * rule on an uncataloged pair still delivers, and `in_app` delivers by
     * default with no catalog entry at all. `enabled` is the answer.
     */
    cataloged: boolean;
}

/**
 * Represents a resolved preference with a target and state.
 */
export interface Preference {
    target: TargetWithLabel;
    state: ResolvedPreferenceState;
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
 * Typed email content for a send. Its presence makes email eligible for this
 * send (direct-only); absence means no email. Bodhveda is a pass-through — the
 * caller renders its own template and passes the result. `subject` is required
 * and at least one of `html`/`text` must be set; `text` is recommended for
 * deliverability and is auto-derived from `html` when omitted.
 */
export interface EmailContent {
    subject: string;
    html?: string;
    text?: string;
}

/**
 * Represents a request to send a notification.
 */
export interface SendNotificationRequest {
    payload: unknown;
    recipient_id?: string;
    target?: Target;
    /**
     * Optional typed email block. Present ⇒ email is attempted (direct sends
     * only); absent ⇒ no email. Gated by catalog + per-medium preference + a
     * primary email contact.
     */
    email?: EmailContent;
}

/**
 * A per-medium delivery outcome returned on a direct send (email in v1).
 */
export interface NotificationDelivery {
    medium: string;
    status: string;
    address?: string;
    failure_reason?: string;
    created_at: string;
    updated_at: string;
}

/**
 * Represents the response after sending a notification.
 */
export interface SendNotificationResponse {
    notification: Notification | null;
    broadcast: Broadcast | null;
    /**
     * Per-medium delivery outcomes for a direct send (email). A partial-medium
     * failure never rejects the send — the outcome is reported here.
     */
    deliveries?: NotificationDelivery[];
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
    /**
     * The medium this preference applies to. Defaults to `in_app` when omitted.
     */
    medium?: PreferenceMedium;
    state: {
        enabled: boolean;
    };
}

/**
 * Represents the response after setting a preference.
 */
export interface SetPreferenceResponse {
    target: TargetWithLabel;
    state: PreferenceState;
}

/**
 * Represents a request to check a preference.
 */
export interface CheckPreferenceRequest {
    target: Target;
    /**
     * The medium to check. Defaults to `in_app` when omitted.
     */
    medium?: PreferenceMedium;
}

/**
 * Represents the response after checking a preference. The target need not be
 * cataloged, or stored at all — any (channel, topic, event) resolves.
 */
export interface CheckPreferenceResponse {
    target: TargetWithLabel;
    state: ResolvedPreferenceState;
}

/**
 * A project-level preference — one entry in the project's CATALOG. The catalog
 * declares which (target, medium) pairs a project may send, and supplies the
 * default a recipient inherits until they override it with a toggle of their own.
 *
 * This is distinct from {@link Preference}, which is one recipient's RESOLVED
 * state. Manage the catalog with `bodhveda.preferences`; manage a recipient's
 * own toggles with `bodhveda.recipients.preferences`.
 */
export interface ProjectPreference {
    id: number;
    project_id: number;
    target: Target;
    /** The medium this catalog entry gates (`in_app` or `email`). */
    medium: PreferenceMedium;
    /**
     * The project-level default for this (target, medium): whether a recipient
     * who has expressed no preference of their own is delivered to.
     */
    default_enabled: boolean;
    label: string;
    created_at: string;
    updated_at: string;
}

/**
 * Request to create ONE catalog entry. Strict: creating an entry whose
 * (channel, topic, event, medium) already exists rejects with a 409 — use
 * {@link ProjectPreferencesClient.update} to change an existing entry, or
 * {@link ProjectPreferencesClient.upsertMany} to declaratively merge a whole
 * catalog.
 */
export interface CreateProjectPreferenceRequest {
    channel: string;
    topic: string;
    event: string;
    /** Defaults to `in_app` when omitted. */
    medium?: PreferenceMedium;
    label: string;
    default_enabled: boolean;
}

/**
 * Request to update a catalog entry. The natural key
 * (channel/topic/event/medium) is immutable, so only the label and default
 * change.
 */
export interface UpdateProjectPreferenceRequest {
    label: string;
    default_enabled: boolean;
}

/**
 * One item of a declarative bulk upsert — the same shape as
 * {@link CreateProjectPreferenceRequest}.
 */
export interface UpsertProjectPreferenceItem
    extends CreateProjectPreferenceRequest {}

/**
 * Options for {@link ProjectPreferencesClient.upsertMany}.
 */
export interface UpsertProjectPreferencesOptions {
    /**
     * When `true`, catalog entries NOT present in the array are DELETED, making
     * the array the project's entire desired catalog. Default `false` (merge):
     * absent entries are left untouched. Pruning un-catalogs a (target, medium),
     * which turns a non-in_app medium off for recipients relying on the catalog
     * default — hence it is opt-in.
     */
    prune?: boolean;
}

/**
 * A delivery transport a recipient contact can be registered for. Only `email`
 * is exercised today; the rest are reserved for future transports.
 */
export type Medium = "email" | "sms" | "web_push" | "mobile_push";

/**
 * Represents a per-medium contact address for a recipient.
 */
export interface RecipientContact {
    id: number;
    medium: Medium;
    address: string;
    is_primary: boolean;
    verified_at: string | null;
    created_at: string;
    updated_at: string;
}

/**
 * Represents a request to add a contact to a recipient.
 */
export interface CreateRecipientContactRequest {
    medium: Medium;
    address: string;
    is_primary?: boolean;
}

/**
 * Represents the response after creating a recipient contact.
 */
export interface CreateRecipientContactResponse extends RecipientContact {}

/**
 * Represents the response after listing a recipient's contacts.
 */
export interface ListRecipientContactsResponse {
    contacts: RecipientContact[];
}

/**
 * Represents a request to update a recipient's contact. Both fields are
 * optional; a changed address invalidates the contact's verification.
 */
export interface UpdateRecipientContactRequest {
    address?: string;
    is_primary?: boolean;
}

/**
 * Represents the response after updating a recipient contact.
 */
export interface UpdateRecipientContactResponse extends RecipientContact {}

/**
 * Request to ensure a primary contact for a medium — the body of
 * {@link RecipientsContactsClient.setPrimary}. Idempotent create-or-update:
 * creates the primary if absent, updates the existing primary's address if it
 * differs (which resets verification), or no-ops if it already matches.
 */
export interface SetPrimaryContactRequest {
    medium: Medium;
    address: string;
}

/**
 * Represents the response after setting a primary contact.
 */
export interface SetPrimaryContactResponse extends RecipientContact {}
