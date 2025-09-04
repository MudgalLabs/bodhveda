import { Target } from "@/features//notification/notification_types";

export type PreferenceKind = "project" | "recipient";

export interface ProjectPreference {
    id: number;
    target: Target;
    label: string;
    default_enabled: boolean;
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
    target: Target;
    recipient_id: string;
    enabled: boolean;
    created_at: string;
    updated_at: string;
}
