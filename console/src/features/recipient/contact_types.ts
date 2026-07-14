export type Medium = "email" | "sms" | "web_push" | "mobile_push";

export interface RecipientContact {
    id: number;
    medium: Medium;
    address: string;
    is_primary: boolean;
    verified_at: string | null;
    created_at: string;
    updated_at: string;
}

export interface CreateRecipientContactPayload {
    medium: Medium;
    address: string;
    is_primary: boolean;
}

export interface UpdateRecipientContactPayload {
    address?: string;
    is_primary?: boolean;
}

export interface ListRecipientContactsResult {
    contacts: RecipientContact[];
}
