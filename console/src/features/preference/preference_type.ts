export type PreferenceKind = "project" | "recipient";

export interface ProjectPreference {
    id: number;
    label: string;
    default_enabled: boolean;
    channel: string;
    topic: string;
    event: string;
    created_at: string;
    updated_at: string;

    subscribers: number;
}

export interface CreateProjectPreferencePayload {
    label: string;
    default_enabled: boolean;
    channel: string;
    event: string | null;
    topic: string | null;
}

export interface RecipientPreference {
    id: number;
    recipient_id: string;
    channel: string;
    topic: string;
    event: string;
    enabled: boolean;
    created_at: string;
    updated_at: string;
}
