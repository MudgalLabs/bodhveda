export interface ProjectPreference {
    id: number;
    label: string;
    default_enabled: boolean;
    channel: string;
    topic: string | null;
    event: string | null;
    created_at: string;
}

export interface CreateProjectPreferencePayload {
    label: string;
    default_enabled: boolean;
    channel: string;
    event: string | null;
    topic: string | null;
}
