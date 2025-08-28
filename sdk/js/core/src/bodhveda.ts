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
}

/**
 * Main class for initializing and interacting with the Bodhveda SDK.
 */
export class Bodhveda implements BodhvedaClient {
    notifications: NotificationsClient;
    recipients: RecipientsClient;

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
}

/**
 * Class for managing recipients.
 */
class Recipients implements RecipientsClient {
    client: AxiosInstance;
    notifications: RecipientsNotificationsClient;
    preferences: RecipientsPreferencesClient;

    /**
     * Creates an instance of the Recipients class.
     * @param client - The Axios client instance.
     */
    constructor(client: AxiosInstance) {
        this.client = client;
        this.notifications = new RecipientsNotifications(client);
        this.preferences = new RecipientsPreferences(client);
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
                params: req.target,
            }
        );
        return response.data as CheckPreferenceResponse;
    }
}
