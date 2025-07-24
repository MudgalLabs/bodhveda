/// <reference types="vite/client" />

interface ImportMetaEnv {
    /**
     * API URL.
     * NOTE: This should be without the API version in the path.
     * @example "https://api.domain.com"
     */
    readonly API_URL: string;

    /**
     * The URL for Google OAuth.
     * This is used for the "Continue with Google" button.
     * @example "https://api.bodhveda.com/v1/platform/auth/oauth/google"
     */
    readonly BODHVEA_GOOGLE_OAUTH_URL: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
