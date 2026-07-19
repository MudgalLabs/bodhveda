// Remembers the last project the user was viewing so the "/" landing resolver
// can drop them back into it instead of always picking the first project.
const LAST_PROJECT_ID_KEY = "bodhveda:last_project_id";

export function setLastProjectId(id: string | number) {
    try {
        localStorage.setItem(LAST_PROJECT_ID_KEY, String(id));
    } catch {
        // Ignore storage failures (e.g. private mode) — this is best-effort.
    }
}

export function getLastProjectId(): string | null {
    try {
        return localStorage.getItem(LAST_PROJECT_ID_KEY);
    } catch {
        return null;
    }
}
