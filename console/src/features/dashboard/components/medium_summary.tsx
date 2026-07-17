import { IconInfo, Tooltip } from "netra";
import { ReactNode } from "react";

import { AnalyticsEmail, AnalyticsInApp } from "@/features/dashboard/analytics_types";
import { formatRate } from "@/features/dashboard/format";
import { OPEN_SOFT_SIGNAL_COPY } from "@/features/notification/delivery_copy";

type Tone = "neutral" | "good" | "bad" | "warn";

function toneClass(tone: Tone, value: number): string {
    const base = "text-2xl font-semibold tabular-nums";
    if (value === 0) return `${base} text-text-muted`;
    switch (tone) {
        case "good":
            return `${base} text-success-foreground`;
        case "bad":
            return `${base} text-error-foreground`;
        case "warn":
            return `${base} text-warning-foreground`;
        default:
            return `${base} text-text-primary`;
    }
}

function Stat({
    label,
    value,
    display,
    tone = "neutral",
    hint,
}: {
    label: string;
    value: number;
    display?: string;
    tone?: Tone;
    hint?: ReactNode;
}) {
    return (
        <div className="flex flex-col gap-y-1">
            <span className="flex-x text-text-muted text-xs">
                {label}
                {hint}
            </span>
            <span className={toneClass(tone, value)}>{display ?? value}</span>
        </div>
    );
}

function Panel({
    title,
    subtitle,
    children,
}: {
    title: string;
    subtitle?: string;
    children: ReactNode;
}) {
    return (
        <div className="border-border-subtle bg-surface-1 flex-1 rounded-md border p-4">
            <div className="mb-3">
                <h3 className="text-text-primary text-sm font-medium">{title}</h3>
                {subtitle && (
                    <p className="text-text-muted mt-0.5 text-xs">{subtitle}</p>
                )}
            </div>
            {children}
        </div>
    );
}

// MediumSummary is the per-medium comparison: in-app on the left, email on the
// right — side by side, NOT on one axis. They are different KINDS of fact (in-app
// `delivered` is a real inbox write; email lives in a separate delivery row and
// its `opened` is a soft signal), so they are never stacked into one bar that
// would imply a shared denominator.
//
// The email panel SELF-HIDES when no email was attempted in range: in-app-only
// projects are the common case and must not see a wall of empty email stats.
export function MediumSummary({
    inApp,
    email,
}: {
    inApp: AnalyticsInApp;
    email: AnalyticsEmail;
}) {
    return (
        <div className="flex flex-col gap-4 lg:flex-row">
            <Panel
                title="In-app"
                subtitle="The inbox outcome — the trustworthy signal."
            >
                <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                    <Stat label="Sent" value={inApp.total} />
                    <Stat
                        label="Delivered"
                        value={inApp.by_status.delivered}
                        tone="good"
                    />
                    <Stat
                        label="Muted"
                        value={inApp.by_status.muted}
                        tone="warn"
                    />
                    <Stat
                        label="Failed"
                        value={inApp.by_status.failed + inApp.by_status.quota_exceeded}
                        tone="bad"
                    />
                </div>
            </Panel>

            {email.attempted > 0 && (
                <Panel
                    title="Email"
                    subtitle="A separate wire send, via the project's provider."
                >
                    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                        <Stat label="Attempted" value={email.attempted} />
                        <Stat
                            label="Delivered"
                            value={email.by_status.delivered}
                            tone="good"
                        />
                        <Stat
                            label="Bounced"
                            value={email.by_status.bounced}
                            tone="bad"
                        />
                        <Stat
                            label="Opened"
                            value={email.opened}
                            hint={
                                <Tooltip content={OPEN_SOFT_SIGNAL_COPY}>
                                    <IconInfo size={12} />
                                </Tooltip>
                            }
                        />
                    </div>
                </Panel>
            )}
        </div>
    );
}

// DeliveryHealth surfaces the numbers that predict a sender-reputation problem —
// the exact risk BYO-first email exists to manage. Rates are over emails the
// provider actually ACCEPTED (sent + delivered + bounced + complained), the
// denominator a mailbox provider judges reputation on; muted/no_contact/pending
// never reached the provider and are excluded.
//
// Thresholds echo the well-known danger zones (a provider like SES starts
// warning near ~5% bounce / ~0.1% complaint). Self-hides with the email panel.
export function DeliveryHealth({ email }: { email: AnalyticsEmail }) {
    if (email.attempted === 0) return null;

    const s = email.by_status;
    const processed = s.sent + s.delivered + s.bounced + s.complained;
    const bounceRate = processed > 0 ? s.bounced / processed : 0;
    const complaintRate = processed > 0 ? s.complained / processed : 0;

    const bounceTone: Tone =
        bounceRate >= 0.05 ? "bad" : bounceRate >= 0.02 ? "warn" : "good";
    const complaintTone: Tone =
        complaintRate >= 0.003 ? "bad" : complaintRate >= 0.001 ? "warn" : "good";

    return (
        <Panel
            title="Delivery health"
            subtitle="Over emails the provider accepted. High rates risk your sender reputation."
        >
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                <Stat
                    label="Bounce rate"
                    value={s.bounced}
                    display={formatRate(s.bounced, processed)}
                    tone={bounceTone}
                    hint={
                        <Tooltip content="Bounced ÷ accepted. Sustained bounce rates above ~5% put your sender reputation — and delivery — at risk.">
                            <IconInfo size={12} />
                        </Tooltip>
                    }
                />
                <Stat
                    label="Complaint rate"
                    value={s.complained}
                    display={formatRate(s.complained, processed)}
                    tone={complaintTone}
                    hint={
                        <Tooltip content="Marked as spam ÷ accepted. Even ~0.1% is a danger sign for most mailbox providers.">
                            <IconInfo size={12} />
                        </Tooltip>
                    }
                />
                <Stat label="Complained" value={s.complained} tone="bad" />
                <Stat label="No contact" value={s.no_contact} tone="warn" />
            </div>
        </Panel>
    );
}
