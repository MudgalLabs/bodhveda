export interface Recipient {
    recipient_id: number;
    name: string;
    created_at: string;
}

export interface CreateRecipientPayload {
    recipient_id: string;
    name: string | null;
}
