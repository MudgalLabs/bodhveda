export interface Recipient {
    id: string;
    name: string;
    created_at: string;
}

export interface CreateRecipientPayload {
    id: string;
    name: string | null;
}

export interface EditRecipientPayload {
    name: string | null;
}

export interface RecipientListItem extends Recipient {
    direct_notifications_count: number;
    broadcast_notifications_count: number;
}

export interface ListRecipientsPayload {
    page?: number;
    limit?: number;
}

export interface ListRecipientsResult {
    recipients: RecipientListItem[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}
