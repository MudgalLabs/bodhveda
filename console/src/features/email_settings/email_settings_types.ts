export type EmailProvider = "resend";

export function emailProviderToString(provider: EmailProvider): string {
    switch (provider) {
        case "resend":
            return "Resend";
        default:
            return "Unknown";
    }
}

// ProjectEmailSettings is the masked representation returned by the API. The
// provider secret is never sent to the client — only `secret_masked` (last 4
// characters).
export interface ProjectEmailSettings {
    provider: EmailProvider;
    from_name: string;
    from_address: string;
    secret_masked: string;
    // Webhook signing secret (Phase 5): masked hint + whether one is configured.
    // Used to verify inbound Resend delivery-status webhooks.
    webhook_secret_masked: string;
    webhook_secret_set: boolean;
    created_at: string;
    updated_at: string;
}

export interface UpsertProjectEmailSettingsPayload {
    provider: EmailProvider;
    from_name: string;
    from_address: string;
    // Optional on update: omit (or leave blank) to keep the existing key and only
    // change the identity/provider. Required on first configuration.
    secret?: string;
    // Always optional: omit/blank to keep the existing webhook secret, or supply a
    // new one to set/rotate it.
    webhook_secret?: string;
}
