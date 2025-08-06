export interface Recipient {
    recipient_id: number;
    name: string;
    created_at: string;
}

export interface CreateRecipientPayload {
    recipient_id: string;
    name: string | null;
}

export interface RecipientListItem extends Recipient {
    direct_notifications_count: number;
    broadcast_notifications_count: number;
}
