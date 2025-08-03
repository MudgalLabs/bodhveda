export type APIKeyScope = "full" | "recipient";

export function apiKeyScopeToString(scope: APIKeyScope): string {
    switch (scope) {
        case "full":
            return "Full Access";
        case "recipient":
            return "Recipient Access";
        default:
            return "Unknown Scope";
    }
}

export interface APIKey {
    id: number;
    name: string;
    token_partial: string;
    scope: APIKeyScope;
    created_at: string;
}

export interface CreateAPIKeyPayload {
    name: string;
    scope: APIKeyScope;
}
