/// <reference types="vite/client" />

interface ImportMetaEnv {
    /**
     * API URL.
     * NOTE: This should be without the API version in the path.
     * @example "https://api.domain.com"
     */
    readonly API_URL: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
