import { Tooltip } from "netra";

import {
    DeliveryOutcome,
    DeliveryTree,
    DeliveryTreeAudience,
    DeliveryTreeMedium,
} from "@/features/notification/notification_types";
import { statusToString } from "@/lib/utils";

// DeliveryTreeView renders how a send actually fanned out:
//
//   audience -> eligible -> medium -> per-status outcomes
//
// It is the same component for a broadcast and a direct send, because a direct
// send IS this tree with a fan-out of one. Keeping one renderer is what makes the
// two comparable at a glance instead of two unrelated screens.

// Outcome presentation. ⚠️ `suppressed` is deliberately NEUTRAL, not an error
// colour: it means the recipient opted out or has no address, which is the system
// working as designed. Painting it red makes a healthy project look broken and
// trains the reader to ignore real failures. Same reasoning for `not_requested` —
// nothing failed, the sender simply never asked for that medium.
const OUTCOME_STYLE: Record<
    DeliveryOutcome,
    { label: string; dot: string; text: string; hint: string }
> = {
    succeeded: {
        label: "Delivered",
        dot: "bg-success-foreground",
        text: "text-success-foreground",
        hint: "Reached the recipient.",
    },
    failed: {
        label: "Failed",
        dot: "bg-error-foreground",
        text: "text-error-foreground",
        hint: "Did not reach the recipient, and should have.",
    },
    suppressed: {
        label: "Suppressed",
        dot: "bg-text-muted",
        text: "text-text-muted",
        hint: "Intentionally not delivered — the recipient opted out, or has no address on file. Not an error.",
    },
    pending: {
        label: "Pending",
        dot: "bg-warning-foreground",
        text: "text-warning-foreground",
        hint: "Still in flight. If this stays non-zero long after the send, the worker has stalled.",
    },
    not_requested: {
        label: "Not requested",
        dot: "bg-text-muted",
        text: "text-text-muted",
        hint: "The sender never asked for this medium — an email-only send has no in-app content.",
    },
};

const OUTCOME_ORDER: DeliveryOutcome[] = [
    "succeeded",
    "pending",
    "suppressed",
    "failed",
    "not_requested",
];

const MEDIUM_LABEL: Record<string, string> = {
    in_app: "In-app",
    email: "Email",
    sms: "SMS",
    web_push: "Web push",
    mobile_push: "Mobile push",
};

function mediumLabel(medium: string) {
    return MEDIUM_LABEL[medium] ?? medium;
}

// Branch is one node of the tree. The left rule plus the short elbow is what
// carries the hierarchy — nesting alone reads as arbitrary indentation.
function Branch({
    children,
    last = false,
}: {
    children: React.ReactNode;
    last?: boolean;
}) {
    return (
        <div className="relative pl-5">
            {/* Vertical rule. Stops halfway down on the last child so the trunk
                visibly ends rather than dangling past the final node. */}
            <span
                className={`border-border-subtle absolute left-0 border-l ${
                    last ? "top-0 h-[0.9rem]" : "inset-y-0"
                }`}
            />
            {/* Elbow into this node. */}
            <span className="border-border-subtle absolute left-0 top-[0.9rem] w-3 border-t" />
            {children}
        </div>
    );
}

function NodeLine({
    label,
    count,
    tone = "",
    hint,
    note,
}: {
    label: React.ReactNode;
    count?: number | string;
    tone?: string;
    hint?: string;
    note?: string;
}) {
    const line = (
        <div className="flex-x flex-wrap items-baseline gap-x-2 py-1">
            <span className={`text-sm ${tone}`}>{label}</span>
            {count !== undefined && (
                <span className="text-text-primary text-sm font-medium tabular-nums">
                    {count}
                </span>
            )}
            {note && (
                <span className="text-text-muted text-xs">{note}</span>
            )}
        </div>
    );

    if (!hint) return line;

    return (
        <Tooltip content={hint}>
            <span className="w-fit cursor-help">{line}</span>
        </Tooltip>
    );
}

// StackedBar gives the split a shape you can read without doing arithmetic.
// Rendered only when there is something to show; a single-bucket bar is noise.
function StackedBar({ outcomes }: { outcomes: Record<string, number> }) {
    const entries = OUTCOME_ORDER.filter(
        (o) => (outcomes[o] ?? 0) > 0
    ).map((o) => [o, outcomes[o]] as [DeliveryOutcome, number]);

    const total = entries.reduce((sum, [, n]) => sum + n, 0);
    if (total === 0 || entries.length < 2) return null;

    return (
        <div className="bg-surface-bg my-1 flex h-1.5 w-full max-w-md overflow-hidden rounded-full">
            {entries.map(([outcome, n]) => (
                <span
                    key={outcome}
                    className={OUTCOME_STYLE[outcome].dot}
                    style={{ width: `${(n / total) * 100}%` }}
                />
            ))}
        </div>
    );
}

function MediumBranch({
    medium,
    last,
}: {
    medium: DeliveryTreeMedium;
    last: boolean;
}) {
    const statuses = Object.entries(medium.statuses).sort(
        ([, a], [, b]) => b - a
    );

    return (
        <Branch last={last}>
            <NodeLine
                label={
                    <span className="text-text-primary font-medium">
                        {mediumLabel(medium.medium)}
                    </span>
                }
                count={medium.total}
                note={medium.total === 1 ? "notification" : "notifications"}
            />

            {medium.total === 0 ? (
                <Branch last>
                    <NodeLine
                        label="Nothing was sent on this medium"
                        tone="text-text-muted"
                    />
                </Branch>
            ) : (
                <>
                    <StackedBar outcomes={medium.outcomes} />

                    {/* Outcome buckets first (the judgement), raw statuses under
                        each (the explanation). */}
                    {OUTCOME_ORDER.filter(
                        (o) => (medium.outcomes[o] ?? 0) > 0
                    ).map((outcome, i, arr) => {
                        const style = OUTCOME_STYLE[outcome];
                        return (
                            <Branch
                                key={outcome}
                                last={i === arr.length - 1}
                            >
                                <NodeLine
                                    label={
                                        <span className="flex-x items-center gap-2">
                                            <span
                                                className={`inline-block size-2 rounded-full ${style.dot}`}
                                            />
                                            <span className={style.text}>
                                                {style.label}
                                            </span>
                                        </span>
                                    }
                                    count={medium.outcomes[outcome]}
                                    hint={style.hint}
                                />
                                {statusChildren(statuses, outcome).map(
                                    ([status, n], j, sub) => (
                                        <Branch
                                            key={status}
                                            last={j === sub.length - 1}
                                        >
                                            <NodeLine
                                                label={statusToString(
                                                    status as never
                                                )}
                                                count={n}
                                                tone="text-text-muted"
                                            />
                                        </Branch>
                                    )
                                )}
                            </Branch>
                        );
                    })}
                </>
            )}
        </Branch>
    );
}

// statusChildren picks the raw statuses worth nesting under an outcome bucket.
//
// ⚠️ It returns NOTHING when the bucket holds a single status whose label already
// matches the bucket's. Otherwise every healthy broadcast renders
// "Delivered 20 → Delivered 20", which reads as a rendering fault rather than a
// breakdown. The children exist to explain a bucket that has more than one cause
// — `Suppressed → Muted / No contact` is the case that earns them.
function statusChildren(
    statuses: [string, number][],
    outcome: DeliveryOutcome
): [string, number][] {
    const inBucket = statuses.filter(
        ([status]) => outcomeForStatus(status) === outcome
    );

    if (inBucket.length === 1) {
        const [status] = inBucket[0];
        if (statusToString(status as never) === OUTCOME_STYLE[outcome].label) {
            return [];
        }
    }

    return inBucket;
}

// outcomeForStatus mirrors enum.Outcome server-side so raw statuses can be nested
// under the bucket they belong to. It is a presentation convenience only — the
// authoritative counts come from the API's `outcomes` map, so a status this does
// not recognise simply renders unnested rather than being miscounted.
function outcomeForStatus(status: string): DeliveryOutcome | null {
    switch (status) {
        case "enqueued":
        case "pending":
        case "sending":
            return "pending";
        case "delivered":
        case "sent":
            return "succeeded";
        case "muted":
        case "no_contact":
        case "suppressed":
            return "suppressed";
        case "failed":
        case "bounced":
        case "complained":
        case "quota_exceeded":
        case "rejected":
            return "failed";
        case "not_requested":
            return "not_requested";
        default:
            return null;
    }
}

function AudienceBranch({ audience }: { audience: DeliveryTreeAudience }) {
    const excluded =
        audience.excluded_disabled + audience.excluded_not_cataloged;

    return (
        <Branch>
            <NodeLine
                label={
                    <span className="text-text-primary font-medium">
                        Audience
                    </span>
                }
                count={audience.total}
                note="recipients at send time"
                hint="Frozen when the broadcast fanned out — not recomputed, so it still reflects the project as it was."
            />

            {excluded > 0 && (
                <Branch>
                    <NodeLine
                        label="Excluded"
                        count={excluded}
                        tone="text-text-muted"
                        // ⚠️ The honest caveat. Excluded recipients are filtered
                        // out before any row is written, so there is genuinely
                        // nothing to open — unlike a direct send, where the same
                        // situation leaves a `muted` row you can inspect.
                        hint={
                            audience.expandable
                                ? undefined
                                : "A count only. These recipients were filtered out before any notification row was written, so there is nothing to drill into."
                        }
                    />

                    {audience.excluded_disabled > 0 && (
                        <Branch last={audience.excluded_not_cataloged === 0}>
                            <NodeLine
                                label="Opted out"
                                count={audience.excluded_disabled}
                                tone="text-text-muted"
                                hint="These recipients turned this target off themselves. Working as intended — nothing to fix."
                            />
                        </Branch>
                    )}

                    {audience.excluded_not_cataloged > 0 && (
                        <Branch last>
                            <NodeLine
                                label="Target not cataloged"
                                count={audience.excluded_not_cataloged}
                                tone="text-warning-foreground"
                                // The actionable one, and the usual reason a
                                // broadcast reaches nobody. Deliberately worded
                                // as a project-config problem rather than a
                                // recipient choice.
                                hint="This project has no enabled in-app catalog entry for this target, so these recipients could never receive it. Add or enable it in Preferences."
                            />
                        </Branch>
                    )}
                </Branch>
            )}

            <Branch last>
                <NodeLine
                    label={
                        <span className="text-text-primary">Eligible</span>
                    }
                    count={audience.eligible}
                    hint="Recipients the broadcast was actually fanned out to."
                />
            </Branch>
        </Branch>
    );
}

export function DeliveryTreeView({ tree }: { tree: DeliveryTree }) {
    return (
        <div className="text-sm">
            {tree.audience ? (
                <AudienceBranch audience={tree.audience} />
            ) : (
                tree.kind === "broadcast" && (
                    <Branch>
                        <NodeLine
                            label="Audience not recorded"
                            tone="text-text-muted"
                            // Distinct from "reached nobody" on purpose: a
                            // broadcast that legitimately matched zero recipients
                            // and one whose audience was never measured are
                            // different facts.
                            hint="This broadcast was sent before audience counts were recorded, or its fan-out has not run yet. This is not the same as reaching nobody."
                        />
                    </Branch>
                )
            )}

            {tree.mediums.map((medium, i) => (
                <MediumBranch
                    key={medium.medium}
                    medium={medium}
                    last={i === tree.mediums.length - 1}
                />
            ))}
        </div>
    );
}
