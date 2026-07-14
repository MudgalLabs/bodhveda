import { IconInfo, Loading, Tooltip } from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useEmailDeliveryOverview } from "@/features/notification/notification_hooks";

// EmailDeliveryOverview renders a compact KPI row of email delivery outcomes for
// the project (Phase 5). Email is DIRECT-only, so this lives above the direct
// notifications table. It stays hidden until at least one email has been
// attempted, so projects not using the email medium see nothing.
export function EmailDeliveryOverview() {
    const projectID = useGetProjectIDFromParams();
    const { data, isLoading } = useEmailDeliveryOverview(projectID);

    const overview = data?.data;

    // Nothing to show until the project has attempted at least one email.
    if (isLoading || !overview || overview.total === 0) {
        return null;
    }

    const stats: {
        label: string;
        value: number;
        tone: "neutral" | "good" | "bad" | "warn";
        soft?: boolean;
    }[] = [
        { label: "Sent", value: overview.sent, tone: "neutral" },
        { label: "Delivered", value: overview.delivered, tone: "good" },
        { label: "Opened", value: overview.opened, tone: "neutral", soft: true },
        { label: "Bounced", value: overview.bounced, tone: "bad" },
        { label: "Complained", value: overview.complained, tone: "bad" },
        { label: "Failed", value: overview.failed, tone: "bad" },
        { label: "No contact", value: overview.no_contact, tone: "warn" },
        { label: "Muted", value: overview.muted, tone: "warn" },
    ];

    return (
        <div className="border-border-subtle bg-surface-1 mb-4 rounded-md border p-4">
            <div className="mb-3 flex items-center gap-x-2">
                <h2 className="text-text-primary text-sm font-medium">
                    Email delivery
                </h2>
                {isLoading && <Loading size={14} />}
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8">
                {stats.map((s) => (
                    <div key={s.label} className="flex flex-col gap-y-1">
                        <span className="flex-x text-text-muted text-xs">
                            {s.label}
                            {s.soft && (
                                <Tooltip content="A soft, directional signal only — open tracking is unreliable (e.g. Apple Mail Privacy Protection pre-fetches images). In-app “read” is the trustworthy signal.">
                                    <IconInfo size={12} />
                                </Tooltip>
                            )}
                        </span>
                        <span className={toneClass(s.tone, s.value)}>
                            {s.value}
                        </span>
                    </div>
                ))}
            </div>
        </div>
    );
}

function toneClass(
    tone: "neutral" | "good" | "bad" | "warn",
    value: number
): string {
    const base = "text-lg font-semibold tabular-nums";
    // A zero count is always muted regardless of tone — nothing to flag.
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
