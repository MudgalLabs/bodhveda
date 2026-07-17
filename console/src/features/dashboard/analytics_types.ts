// Mirrors dto.ProjectAnalytics (Phase 9.5). In-app and email are aggregated over
// DIFFERENT tables — the in-app status scalar on `notification`, the email
// outcome on `notification_delivery` — so they are two shapes, never one. See
// the API's dto.ProjectAnalytics for why a join would be wrong.

export interface AnalyticsInAppByStatus {
    enqueued: number;
    muted: number;
    delivered: number;
    quota_exceeded: number;
    failed: number;
}

export interface AnalyticsInAppDay {
    day: string; // YYYY-MM-DD, in the viewer's timezone
    total: number;
    enqueued: number;
    muted: number;
    delivered: number;
    quota_exceeded: number;
    failed: number;
}

export interface AnalyticsInApp {
    total: number;
    by_status: AnalyticsInAppByStatus;
    series: AnalyticsInAppDay[];
}

export interface AnalyticsEmailByStatus {
    pending: number;
    sent: number;
    delivered: number;
    bounced: number;
    complained: number;
    failed: number;
    no_contact: number;
    muted: number;
}

export interface AnalyticsEmailDay {
    day: string;
    attempted: number;
    delivered: number;
    bounced: number;
    complained: number;
}

// `opened` / `clicked` are SOFT, directional signals (Apple MPP inflates opens).
// Never present them as the trustworthy in-app `read`. attempted === 0 is the
// self-hiding signal: a project that has sent no email in range shows no email
// charts.
export interface AnalyticsEmail {
    attempted: number;
    by_status: AnalyticsEmailByStatus;
    opened: number;
    clicked: number;
    series: AnalyticsEmailDay[];
}

export interface AnalyticsTargetStat {
    channel: string;
    topic: string;
    event: string;
    notifications: number;
    email_attempted: number;
    email_delivered: number;
    email_bounced: number;
    email_complained: number;
}

export interface ProjectAnalytics {
    in_app: AnalyticsInApp;
    email: AnalyticsEmail;
    targets: AnalyticsTargetStat[];
}

// The query params the analytics endpoint accepts: an absolute RFC3339 range
// derived from the picked calendar days (the API decodes them into *time.Time,
// so a blank param is a hard 400 — omit, never blank).
export interface ProjectAnalyticsParams {
    created_from?: string;
    created_to?: string;
}
