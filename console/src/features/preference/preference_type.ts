import { Target } from "@/features//notification/notification_types";

export type PreferenceKind = "project" | "recipient";

// Active preference mediums in v1. The backend enum scaffolds sms/web_push/
// mobile_push too, but only these can be cataloged/toggled today.
export type PreferenceMedium = "in_app" | "email";

export const PREFERENCE_MEDIUMS: PreferenceMedium[] = ["in_app", "email"];

export const PREFERENCE_MEDIUM_LABELS: Record<PreferenceMedium, string> = {
    in_app: "In-App",
    email: "Email",
};

export function mediumLabel(medium: string): string {
    return PREFERENCE_MEDIUM_LABELS[medium as PreferenceMedium] ?? medium;
}

export interface ProjectPreference {
    id: number;
    target: Target;
    medium: PreferenceMedium;
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
    medium: PreferenceMedium;
}

export interface RecipientPreference {
    id: number;
    target: Target;
    medium: PreferenceMedium;
    recipient_id: string;
    enabled: boolean;
    created_at: string;
    updated_at: string;
}
