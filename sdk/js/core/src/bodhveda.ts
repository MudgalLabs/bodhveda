import axios, { AxiosError, AxiosInstance } from "axios";

import {
    ListNotificationsResponse,
    ListNotificationsRequest,
    ListPreferencesResponse,
    SetPreferenceRequest,
    SetPreferenceResponse,
    UnreadCountResponse,
    UpdateNotificationsStateResponse,
    UpdateNotificationsStateRequest,
    DeleteNotificationsRequest,
    DeleteNotificationsResponse,
    CheckPreferenceRequest,
    CheckPreferenceResponse,
    SendNotificationRequest,
    SendNotificationResponse,
    CreateRecipientRequest,
    CreateRecipientResponse,
    CreateRecipientsBatchRequest,
    CreateRecipientsBatchResponse,
    GetRecipientResponse,
    UpdateRecipientRequest,
    UpdateRecipientResponse,
    CreateRecipientContactRequest,
    CreateRecipientContactResponse,
    ListRecipientContactsResponse,
    UpdateRecipientContactRequest,
    UpdateRecipientContactResponse,
    SetPrimaryContactRequest,
    SetPrimaryContactResponse,
    ProjectPreference,
    CreateProjectPreferenceRequest,
    UpdateProjectPreferenceRequest,
    UpsertProjectPreferenceItem,
    UpsertProjectPreferencesOptions,
} from "./types";
import { ROUTES } from "./routes";

/**
 * Options for configuring the Bodhveda SDK.
 */
interface BodhvedaOptions {
    apiURL?: string;
}

/**
 * Main interface for interacting with Bodhveda services.
 */
interface BodhvedaClient {
    /**
     * Provides access to notification-related methods.
     */
    notifications: NotificationsClient;
    /**
     * Provides access to recipient-related methods.
     */
    recipients: RecipientsClient;
    /**
     * Provides access to the project preference CATALOG (project-scoped by the
     * API key). Distinct from `recipients.preferences`, which manages a single
     * recipient's own toggles.
     */
    preferences: ProjectPreferencesClient;
}

/**
 * Main class for initializing and interacting with the Bodhveda SDK.
 */
export class Bodhveda implements BodhvedaClient {
    notifications: NotificationsClient;
    recipients: RecipientsClient;
    preferences: ProjectPreferencesClient;

    /**
     * Creates an instance of the Bodhveda SDK.
     * @param apiKey - The API key for authentication.
     * @param options - Optional configuration options.
     */
    constructor(apiKey: string, options: BodhvedaOptions = {}) {
        const { apiURL = "https://api.bodhveda.com" } = options;

        const client = axios.create({
            baseURL: apiURL,
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${apiKey}`,
            },
        });

        client.interceptors.response.use(
            (response) => response.data, // Unwrap the main data object from the response.
            (error: AxiosError) => {
                if (axios.isAxiosError(error) && error.response?.data) {
                    throw error.response?.data; // Throw the API's error response directly.
                } else {
                    throw error;
                }
            }
        );

        this.notifications = new Notifications(client);
        this.recipients = new Recipients(client);
        this.preferences = new ProjectPreferences(client);
    }
}

/**
 * Interface for interacting with notifications.
 */
interface NotificationsClient {
    /**
     * Sends a notification.
     * @param req - The request object containing notification details.
     * @returns The response from the API.
     */
    send(req: SendNotificationRequest): Promise<SendNotificationResponse>;
}

/**
 * Class for managing notifications.
 */
class Notifications implements NotificationsClient {
    client: AxiosInstance;

    /**
     * Creates an instance of the Notifications class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
    }

    async send(
        req: SendNotificationRequest
    ): Promise<SendNotificationResponse> {
        const response = await this.client.post(ROUTES.notifications.send, req);
        return response.data as SendNotificationResponse;
    }
}

/**
 * Interface for interacting with recipients.
 */
interface RecipientsClient {
    /**
     * Creates a new recipient.
     * @param req - The request object containing recipient details.
     * @returns The response after creating the recipient.
     */
    create(req: CreateRecipientRequest): Promise<CreateRecipientResponse>;

    /**
     * Creates multiple recipients in a batch.
     * @param req - The request object containing an array of recipients.
     * @returns The response after creating recipients in batch.
     */
    createBatch(
        req: CreateRecipientsBatchRequest
    ): Promise<CreateRecipientsBatchResponse>;

    /**
     * Retrieves a recipient by ID.
     * @param recipientID - The unique identifier of the recipient.
     * @returns The recipient details.
     */
    get(recipientID: string): Promise<GetRecipientResponse>;

    /**
     * Updates a recipient by ID.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object containing updated recipient details.
     * @returns The updated recipient details.
     */
    update(
        recipientID: string,
        req: UpdateRecipientRequest
    ): Promise<UpdateRecipientResponse>;

    /**
     * Deletes a recipient by ID.
     * @param recipientID - The unique identifier of the recipient.
     * @returns A promise that resolves when the recipient is deleted.
     */
    delete(recipientID: string): Promise<void>;

    /**
     * Provides access to recipient preferences methods.
     */
    preferences: RecipientsPreferencesClient;

    /**
     * Provides access to recipient notifications methods.
     */
    notifications: RecipientsNotificationsClient;

    /**
     * Provides access to recipient contacts methods.
     */
    contacts: RecipientsContactsClient;
}

/**
 * Class for managing recipients.
 */
class Recipients implements RecipientsClient {
    client: AxiosInstance;
    notifications: RecipientsNotificationsClient;
    preferences: RecipientsPreferencesClient;
    contacts: RecipientsContactsClient;

    /**
     * Creates an instance of the Recipients class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
        this.notifications = new RecipientsNotifications(client);
        this.preferences = new RecipientsPreferences(client);
        this.contacts = new RecipientsContacts(client);
    }

    async create(
        req: CreateRecipientRequest
    ): Promise<CreateRecipientResponse> {
        const response = await this.client.post(ROUTES.recipients.create, req);
        return response.data as CreateRecipientResponse;
    }

    async createBatch(
        req: CreateRecipientsBatchRequest
    ): Promise<CreateRecipientsBatchResponse> {
        const response = await this.client.post(
            ROUTES.recipients.createBatch,
            req
        );
        return response.data as CreateRecipientsBatchResponse;
    }

    async get(recipientID: string): Promise<GetRecipientResponse> {
        const response = await this.client.get(
            ROUTES.recipients.get(recipientID)
        );
        return response.data as GetRecipientResponse;
    }

    async update(
        recipientID: string,
        req: UpdateRecipientRequest
    ): Promise<UpdateRecipientResponse> {
        const response = await this.client.patch(
            ROUTES.recipients.update(recipientID),
            req
        );
        return response.data as UpdateRecipientResponse;
    }

    async delete(recipientID: string): Promise<void> {
        await this.client.delete(ROUTES.recipients.delete(recipientID));
    }
}

/**
 * Interface for managing recipient notifications.
 */
interface RecipientsNotificationsClient {
    /**
     * Lists notifications for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - Optional request parameters for listing notifications.
     * @returns The response containing the list of notifications.
     */
    list(
        recipientID: string,
        req?: ListNotificationsRequest
    ): Promise<ListNotificationsResponse>;

    /**
     * Gets the count of unread notifications for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @returns The response containing the unread count.
     */
    unreadCount(recipientID: string): Promise<UnreadCountResponse>;

    /**
     * Updates the state of notifications for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object containing state updates.
     * @returns The response after updating notification states.
     */
    updateState(
        recipientID: string,
        req: UpdateNotificationsStateRequest
    ): Promise<UpdateNotificationsStateResponse>;

    /**
     * Deletes notifications for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object containing IDs of notifications to delete.
     * @returns The response after deleting notifications.
     */
    delete(
        recipientID: string,
        req: DeleteNotificationsRequest
    ): Promise<DeleteNotificationsResponse>;
}

/**
 * Class for managing recipient notifications.
 */
class RecipientsNotifications implements RecipientsNotificationsClient {
    client: AxiosInstance;

    /**
     * Creates an instance of the RecipientsNotifications class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
    }

    async list(
        recipientID: string,
        req?: ListNotificationsRequest
    ): Promise<ListNotificationsResponse> {
        const response = await this.client.get(
            ROUTES.recipients.notifications.list(recipientID),
            {
                params: req,
            }
        );
        return response.data as ListNotificationsResponse;
    }

    async unreadCount(recipientID: string): Promise<UnreadCountResponse> {
        const response = await this.client.get(
            ROUTES.recipients.notifications.unreadCount(recipientID)
        );
        return response.data as UnreadCountResponse;
    }

    async updateState(
        recipientID: string,
        req: UpdateNotificationsStateRequest
    ): Promise<UpdateNotificationsStateResponse> {
        const response = await this.client.patch(
            ROUTES.recipients.notifications.udpateState(recipientID),
            req
        );
        return response.data as UpdateNotificationsStateResponse;
    }

    async delete(
        recipientID: string,
        req: DeleteNotificationsRequest
    ): Promise<DeleteNotificationsResponse> {
        const response = await this.client.delete(
            ROUTES.recipients.notifications.delete(recipientID),
            {
                data: req,
            }
        );
        return response.data as DeleteNotificationsResponse;
    }
}

/**
 * Interface for managing recipient preferences.
 */
interface RecipientsPreferencesClient {
    /**
     * Lists preferences for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @returns The response containing the list of preferences.
     */
    list(recipientID: string): Promise<ListPreferencesResponse>;

    /**
     * Sets a preference for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object containing the preference to set.
     * @returns The response after setting the preference.
     */
    set(
        recipientID: string,
        req: SetPreferenceRequest
    ): Promise<SetPreferenceResponse>;

    /**
     * Checks a preference for a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object specifying the preference to check.
     * @returns The response after checking the preference.
     */
    check(
        recipientID: string,
        req: CheckPreferenceRequest
    ): Promise<CheckPreferenceResponse>;
}

/**
 * Class for managing recipient preferences.
 */
class RecipientsPreferences implements RecipientsPreferencesClient {
    client: AxiosInstance;

    /**
     * Creates an instance of the RecipientsPreferences class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
    }

    async list(recipientID: string): Promise<ListPreferencesResponse> {
        const response = await this.client.get(
            ROUTES.recipients.preferences.list(recipientID)
        );
        return response.data as ListPreferencesResponse;
    }

    async set(
        recipientID: string,
        req: SetPreferenceRequest
    ): Promise<SetPreferenceResponse> {
        const response = await this.client.patch(
            ROUTES.recipients.preferences.set(recipientID),
            {
                target: req.target,
                medium: req.medium,
                state: req.state,
            }
        );
        return response.data as SetPreferenceResponse;
    }

    async check(
        recipientID: string,
        req: CheckPreferenceRequest
    ): Promise<CheckPreferenceResponse> {
        const response = await this.client.get(
            ROUTES.recipients.preferences.check(recipientID),
            {
                params: { ...req.target, medium: req.medium },
            }
        );
        return response.data as CheckPreferenceResponse;
    }
}

/**
 * Interface for managing recipient contacts.
 */
interface RecipientsContactsClient {
    /**
     * Lists a recipient's contacts.
     * @param recipientID - The unique identifier of the recipient.
     * @returns The response containing the list of contacts.
     */
    list(recipientID: string): Promise<ListRecipientContactsResponse>;

    /**
     * Adds a contact to a recipient.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The request object containing the contact to add.
     * @returns The response after creating the contact.
     */
    create(
        recipientID: string,
        req: CreateRecipientContactRequest
    ): Promise<CreateRecipientContactResponse>;

    /**
     * Ensures an address is the recipient's PRIMARY contact for a medium
     * (idempotent create-or-update). Use this for a server-side sync that keeps
     * a recipient's primary email current — it is a single call, unlike
     * {@link create} which rejects (409) when the contact already exists.
     * @param recipientID - The unique identifier of the recipient.
     * @param req - The medium and address to make primary.
     * @returns The resulting primary contact.
     */
    setPrimary(
        recipientID: string,
        req: SetPrimaryContactRequest
    ): Promise<SetPrimaryContactResponse>;

    /**
     * Updates a recipient's contact by contact ID.
     * @param recipientID - The unique identifier of the recipient.
     * @param contactID - The unique identifier of the contact.
     * @param req - The request object containing the fields to update.
     * @returns The response after updating the contact.
     */
    update(
        recipientID: string,
        contactID: number,
        req: UpdateRecipientContactRequest
    ): Promise<UpdateRecipientContactResponse>;

    /**
     * Deletes a recipient's contact by contact ID. Requires a full-scope API key.
     * @param recipientID - The unique identifier of the recipient.
     * @param contactID - The unique identifier of the contact.
     * @returns A promise that resolves when the contact is deleted.
     */
    delete(recipientID: string, contactID: number): Promise<void>;
}

/**
 * Class for managing recipient contacts.
 */
class RecipientsContacts implements RecipientsContactsClient {
    client: AxiosInstance;

    /**
     * Creates an instance of the RecipientsContacts class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
    }

    async list(recipientID: string): Promise<ListRecipientContactsResponse> {
        const response = await this.client.get(
            ROUTES.recipients.contacts.list(recipientID)
        );
        return response.data as ListRecipientContactsResponse;
    }

    async create(
        recipientID: string,
        req: CreateRecipientContactRequest
    ): Promise<CreateRecipientContactResponse> {
        const response = await this.client.post(
            ROUTES.recipients.contacts.create(recipientID),
            req
        );
        return response.data as CreateRecipientContactResponse;
    }

    async setPrimary(
        recipientID: string,
        req: SetPrimaryContactRequest
    ): Promise<SetPrimaryContactResponse> {
        const response = await this.client.put(
            ROUTES.recipients.contacts.setPrimary(recipientID),
            req
        );
        return response.data as SetPrimaryContactResponse;
    }

    async update(
        recipientID: string,
        contactID: number,
        req: UpdateRecipientContactRequest
    ): Promise<UpdateRecipientContactResponse> {
        const response = await this.client.patch(
            ROUTES.recipients.contacts.update(recipientID, contactID),
            req
        );
        return response.data as UpdateRecipientContactResponse;
    }

    async delete(recipientID: string, contactID: number): Promise<void> {
        await this.client.delete(
            ROUTES.recipients.contacts.delete(recipientID, contactID)
        );
    }
}

/**
 * Interface for managing the project preference CATALOG — the project-level
 * entries that declare which (target, medium) pairs the project may send and the
 * default a recipient inherits. Project-scoped by the API key.
 *
 * Not to be confused with `recipients.preferences`, which manages one
 * recipient's own toggles.
 */
interface ProjectPreferencesClient {
    /**
     * Lists the project's catalog.
     * @returns The catalog entries.
     */
    list(): Promise<ProjectPreference[]>;

    /**
     * Retrieves a single catalog entry by ID.
     * @param preferenceID - The catalog entry's ID.
     * @returns The catalog entry.
     */
    get(preferenceID: number): Promise<ProjectPreference>;

    /**
     * Creates a single catalog entry. Strict — rejects with a 409 when an entry
     * for the same (channel, topic, event, medium) already exists. To change an
     * existing entry use {@link update}; to declaratively set a whole catalog use
     * {@link upsertMany}.
     * @param req - The catalog entry to create.
     * @returns The created catalog entry.
     */
    create(req: CreateProjectPreferenceRequest): Promise<ProjectPreference>;

    /**
     * Updates a catalog entry's label and default. The natural key
     * (channel/topic/event/medium) is immutable.
     * @param preferenceID - The catalog entry's ID.
     * @param req - The fields to update.
     * @returns The updated catalog entry.
     */
    update(
        preferenceID: number,
        req: UpdateProjectPreferenceRequest
    ): Promise<ProjectPreference>;

    /**
     * Deletes a catalog entry (un-catalogs the (target, medium)).
     * @param preferenceID - The catalog entry's ID.
     */
    delete(preferenceID: number): Promise<void>;

    /**
     * Declaratively merges a whole catalog in one call — the primitive for a
     * one-off "set up my project's preferences" script. Each item is upserted by
     * its natural key (new inserted, existing label + default updated). By default
     * entries absent from the array are left untouched; pass `{ prune: true }` to
     * also delete them, making the array the entire desired catalog.
     * @param prefs - The desired catalog entries.
     * @param options - Set `prune: true` to remove entries absent from the array.
     * @returns The full resulting catalog.
     */
    upsertMany(
        prefs: UpsertProjectPreferenceItem[],
        options?: UpsertProjectPreferencesOptions
    ): Promise<ProjectPreference[]>;
}

/**
 * Class for managing the project preference catalog.
 */
class ProjectPreferences implements ProjectPreferencesClient {
    client: AxiosInstance;

    /**
     * Creates an instance of the ProjectPreferences class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
    }

    async list(): Promise<ProjectPreference[]> {
        const response = await this.client.get(ROUTES.preferences.list);
        return response.data as ProjectPreference[];
    }

    async get(preferenceID: number): Promise<ProjectPreference> {
        const response = await this.client.get(
            ROUTES.preferences.get(preferenceID)
        );
        return response.data as ProjectPreference;
    }

    async create(
        req: CreateProjectPreferenceRequest
    ): Promise<ProjectPreference> {
        const response = await this.client.post(
            ROUTES.preferences.create,
            req
        );
        return response.data as ProjectPreference;
    }

    async update(
        preferenceID: number,
        req: UpdateProjectPreferenceRequest
    ): Promise<ProjectPreference> {
        const response = await this.client.patch(
            ROUTES.preferences.update(preferenceID),
            req
        );
        return response.data as ProjectPreference;
    }

    async delete(preferenceID: number): Promise<void> {
        await this.client.delete(ROUTES.preferences.delete(preferenceID));
    }

    async upsertMany(
        prefs: UpsertProjectPreferenceItem[],
        options?: UpsertProjectPreferencesOptions
    ): Promise<ProjectPreference[]> {
        const response = await this.client.put(
            ROUTES.preferences.upsertMany,
            prefs,
            options?.prune ? { params: { prune: true } } : undefined
        );
        return response.data as ProjectPreference[];
    }
}
