import { Target } from "@/features//notification/notification_types";

export const PREFERENCE_KINDS = ["project", "recipient"] as const;

export type PreferenceKind = (typeof PREFERENCE_KINDS)[number];

export const DEFAULT_PREFERENCE_KIND: PreferenceKind = "project";

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
    name: string;
    /** Optional longer blurb; null when unset. */
    description: string | null;
    default_enabled: boolean;
    /**
     * Recipients cannot opt out of this entry. It is still cataloged (so sends
     * pass the target gate) but the recipient toggle is refused, and their own
     * rule is ignored by the resolution cascade. `default_enabled` still
     * applies, so it remains the project's way to stop sending.
     */
    mandatory: boolean;
    created_at: string;
    updated_at: string;

    subscribers: number;
}

export interface CreateProjectPreferencePayload {
    name: string;
    /** Optional; omitted or blank stores no description. */
    description?: string;
    default_enabled: boolean;
    channel: string;
    event: string | null;
    topic: string | null;
    medium: PreferenceMedium;
}

// Only the mutable fields of a catalog entry. The natural key (channel, topic,
// event, medium) is immutable server-side, so an edit changes just these.
export interface UpdateProjectPreferencePayload {
    name: string;
    /** Optional; omitted or blank clears the description. */
    description?: string;
    default_enabled: boolean;
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

/**
 * Which rung of the resolution cascade decided a cell's value.
 *
 * The cascade is: recipient-exact → recipient topic='any' → project-exact →
 * project topic='any' → the medium-dependent default.
 */
export type PreferenceSource =
    | "recipient_exact"
    | "recipient_any"
    | "project_exact"
    | "project_any"
    | "default";

/**
 * One recipient's RESOLVED state for a (target, medium) — what a send would
 * ACTUALLY do, resolved server-side by the same cascade the send path uses.
 *
 * Read `enabled` as the only honest field. In particular `cataloged` is NOT a
 * gate: the catalog is a default, and an explicit recipient row on an
 * uncataloged pair still delivers because it wins the cascade first. Rendering
 * "unavailable" off `cataloged` would be a lie — it was, before Phase 9.3.
 */
export interface RecipientPreferenceTargetState {
    target: Target & {
        medium: PreferenceMedium;
        /** The catalog entry's name. Omitted by the API when unset. */
        name?: string;
        /** The catalog entry's optional description, when it has one. */
        description?: string;
    };
    state: {
        /** The resolved decision. Agrees with the send path's gating. */
        enabled: boolean;
        /**
         * True when the recipient has no row of their own for this exact
         * (target, medium) — the value came from elsewhere in the cascade.
         * Toggling writes exactly that missing row.
         */
        inherited: boolean;
        /**
         * A project-level row exists for this exact (target, medium). Context
         * for explaining a cell, never a gate on it.
         */
        cataloged: boolean;
        /**
         * The catalog entry deciding this cell is MANDATORY — the recipient
         * cannot opt out. Unlike `cataloged` this one does decide `enabled`,
         * because a mandatory entry outranks the recipient's own rule, so the
         * toggle must be rendered locked. Writing one is refused with a 400.
         */
        mandatory: boolean;
        source: PreferenceSource;
    };
}

export interface RecipientPreferenceTargetStatesResult {
    preferences: RecipientPreferenceTargetState[];
}

/**
 * Body of the console's recipient-preference PUT. Deliberately FLAT and
 * one-(target, medium)-per-call — the existing shape, reused as-is.
 *
 * This write converges with the Developer API's PATCH and the email one-click
 * unsubscribe at the repository layer (all three upsert the same
 * (project, recipient, target, medium) row). That convergence is why an
 * unsubscribe and this toggle stay in sync; do not route around it.
 */
export interface UpsertRecipientPreferencePayload {
    channel: string;
    topic: string;
    event: string;
    medium: PreferenceMedium;
    enabled: boolean;
}

/**
 * One (target, medium) this project has sent but never cataloged — a send that
 * strict targets would have rejected.
 */
export interface UncatalogedTarget {
    channel: string;
    topic: string;
    event: string;
    medium: PreferenceMedium;
    /** Sends in the window that named it. A broadcast counts once, not once per recipient. */
    sends: number;
    last_sent_at: string;
}

/**
 * What strict targets would reject for this project, over a trailing window.
 *
 * This is the reason the setting is findable at all: it defaults to off, and a
 * flag nobody discovers is a flag nobody enables. It also reads usefully for a
 * project that never intends to turn the gate on, because an uncataloged target
 * has no preference surface — recipients can't see it, so they can't mute it.
 */
export interface CatalogDriftResult {
    since: string;
    total_sends: number;
    targets: UncatalogedTarget[];
}
