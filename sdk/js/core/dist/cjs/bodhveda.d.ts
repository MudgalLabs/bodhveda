import { ListNotificationsResponse, ListNotificationsRequest, ListPreferencesResponse, SetPreferenceRequest, SetPreferenceResponse, UnreadCountResponse, UpdateNotificationsStateResponse, UpdateNotificationsStateRequest, DeleteNotificationsRequest, DeleteNotificationsResponse, CheckPreferenceRequest, CheckPreferenceResponse, SendNotificationRequest, SendNotificationResponse, CreateRecipientRequest, CreateRecipientResponse, CreateRecipientsBatchRequest, CreateRecipientsBatchResponse, GetRecipientResponse, UpdateRecipientRequest, UpdateRecipientResponse } from "./types";
interface BodhvedaOptions {
    apiURL?: string;
}
interface BodhvedaAPI {
    notifications: NotificationsAPI;
    recipients: RecipientsAPI;
}
export declare class Bodhveda implements BodhvedaAPI {
    notifications: NotificationsAPI;
    recipients: RecipientsAPI;
    constructor(apiKey: string, options?: BodhvedaOptions);
}
interface NotificationsAPI {
    send: (req: SendNotificationRequest) => Promise<SendNotificationResponse>;
}
interface RecipientsAPI {
    create: (req: CreateRecipientRequest) => Promise<CreateRecipientResponse>;
    createBatch: (req: CreateRecipientsBatchRequest) => Promise<CreateRecipientsBatchResponse>;
    get: (recipientID: string) => Promise<GetRecipientResponse>;
    update: (recipientID: string, req: UpdateRecipientRequest) => Promise<UpdateRecipientResponse>;
    delete: (recipientID: string) => Promise<void>;
    preferences: RecipientsPreferencesAPI;
    notifications: RecipientsNotificationsAPI;
}
interface RecipientsNotificationsAPI {
    list(recipientID: string, req?: ListNotificationsRequest): Promise<ListNotificationsResponse>;
    unreadCount(recipientID: string): Promise<UnreadCountResponse>;
    updateState(recipientID: string, req: UpdateNotificationsStateRequest): Promise<UpdateNotificationsStateResponse>;
    delete(recipientID: string, req: DeleteNotificationsRequest): Promise<DeleteNotificationsResponse>;
}
interface RecipientsPreferencesAPI {
    list(recipientID: string): Promise<ListPreferencesResponse>;
    set(recipientID: string, req: SetPreferenceRequest): Promise<SetPreferenceResponse>;
    check(recipientID: string, req: CheckPreferenceRequest): Promise<CheckPreferenceResponse>;
}
export {};
//# sourceMappingURL=bodhveda.d.ts.map