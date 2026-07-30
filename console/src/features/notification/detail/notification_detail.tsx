import {
    ErrorMessage,
    IconArrowLeft,
    Loading,
    Tooltip,
    formatDate,
    formatDuration,
} from "netra";
import { Link } from "@tanstack/react-router";
import { ReactNode, useMemo } from "react";

import { DeliveryTreeView } from "@/features/notification/components/delivery_tree";
import { StatusTag } from "@/components/status_tag";
import {
    Verdict,
    VerdictTone,
    verdictFor,
} from "@/features/notification/detail/delivery_verdict";
import {
    useBroadcast,
    useBroadcastDeliveryTree,
    useNotification,
    useNotificationDeliveries,
    useNotificationDeliveryTree,
} from "@/features/notification/notification_hooks";
import {
    Broadcast,
    DeliveryTree,
    Notification,
    NotificationDeliveryDetail,
} from "@/features/notification/notification_types";
import {
    EVENT_KIND_LABEL,
    OPEN_SOFT_SIGNAL_COPY,
    deliveryOutcomeText,
} from "@/features/notification/delivery_copy";
import { RecipientLink } from "@/features/recipient/recipient_link";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { targetToString } from "@/lib/utils";

// The notification detail PAGE — one shell for a direct send and a broadcast.
//
// Why a page and not just the modals it supplements: a notification id is the
// natural thing to paste into an incident note, and a modal cannot be linked.
// This surface exists for debugging ("here is the one that failed"), so a stable
// URL is the point rather than a nicety.
//
// ⚠️ Direct and broadcast get SEPARATE routes (/notifications/:id and
// /broadcasts/:id) because notification.id and broadcast.id are independent
// SERIAL sequences whose ranges overlap — measured 2026-07-30: notifications
// 1–750563, broadcasts 23–31. One path could not tell /notifications/25 apart.
// They share this component, so the two never drift into unrelated screens.
//
// LAYOUT: the verdict leads, the evidence follows. The first version of this page
// was a flat run of label/value pairs with the tree floating under it, which meant
// the reader had to compute the outcome themselves from five nested counts. The
// verdict band states the conclusion; the fan-out proves it; the rail holds the
// metadata you only want once you have a reason to care.

const TONE_STYLE: Record<VerdictTone, { rail: string; text: string }> = {
    success: { rail: "bg-success-foreground", text: "text-success-foreground" },
    warning: { rail: "bg-warning-foreground", text: "text-warning-foreground" },
    error: { rail: "bg-error-foreground", text: "text-error-foreground" },
    neutral: { rail: "bg-text-muted", text: "text-text-primary" },
};

// VerdictBand is this page's signature: the answer, stated, before any evidence.
function VerdictBand({ verdict }: { verdict: Verdict }) {
    const tone = TONE_STYLE[verdict.tone];

    return (
        <div className="flex gap-4">
            {/* The rail carries the outcome as colour, so the verdict reads before
                any of the words do. It is the only saturated element on the page. */}
            <span
                aria-hidden
                className={`w-[3px] shrink-0 rounded-full ${tone.rail}`}
            />
            <div className="min-w-0 py-1">
                <p
                    className={`text-2xl leading-tight font-medium tabular-nums ${tone.text}`}
                >
                    {verdict.headline}
                </p>
                {verdict.unit && (
                    <p className="text-text-muted mt-0.5 text-sm">
                        {verdict.unit}
                    </p>
                )}
                {verdict.detail && (
                    <p className="text-text-primary mt-2 max-w-prose text-sm">
                        {verdict.detail}
                    </p>
                )}
            </div>
        </div>
    );
}

// Eyebrow labels. Uppercase micro-type is the structural device here because the
// page has exactly three kinds of region and they need separating without another
// box or rule competing with the verdict rail.
function Eyebrow({ children }: { children: ReactNode }) {
    return (
        <h2 className="text-text-muted mb-3 text-[11px] font-medium tracking-[0.08em] uppercase">
            {children}
        </h2>
    );
}

function Card({ children }: { children: ReactNode }) {
    return (
        <div className="border-border-subtle bg-surface-bg/40 rounded-lg border p-4">
            {children}
        </div>
    );
}

interface Fact {
    label: string;
    value: ReactNode;
    hint?: string;
}

function FactList({ facts }: { facts: Fact[] }) {
    return (
        <dl className="space-y-3 text-sm">
            {facts.map(({ label, value, hint }) => (
                <div key={label} className="flex flex-col gap-0.5">
                    <dt className="text-text-muted text-xs">
                        {hint ? (
                            <Tooltip content={hint}>
                                <span className="cursor-help underline decoration-dotted underline-offset-2">
                                    {label}
                                </span>
                            </Tooltip>
                        ) : (
                            label
                        )}
                    </dt>
                    <dd className="text-text-primary">{value}</dd>
                </div>
            ))}
        </dl>
    );
}

function Mono({ children }: { children: ReactNode }) {
    return (
        <span className="select-text! font-mono text-[13px]">{children}</span>
    );
}

function PayloadBlock({ payload }: { payload: unknown }) {
    // A payload-less send is normal (email-only), so say so rather than showing an
    // empty box that reads like a loading state.
    if (payload === null || payload === undefined) {
        return (
            <p className="text-text-muted text-sm">
                No in-app payload — this send did not request in-app delivery.
            </p>
        );
    }

    // ⚠️ index.css styles `pre` globally as an inline-block chip with its own
    // background, padding and radius. Wrapping it in another card produced a box
    // inside a box, sized to the content instead of the column. So the <pre> IS
    // the container here — overridden to block/full-width and scrollable.
    //
    // The `!` is needed because netra's stylesheet loads after the console's
    // Tailwind and wins on source order at equal specificity. See
    // agent-docs/overview.md.
    return (
        <pre className="text-text-primary! block! max-h-72 w-full! max-w-full! overflow-auto p-4! text-xs! leading-relaxed select-text!">
            {JSON.stringify(payload, null, 2)}
        </pre>
    );
}

// Delivery statuses that mean the provider was never contacted, so no provider
// event can ever arrive for this row.
const NEVER_SENT_STATUSES = new Set<string>([
    "muted",
    "no_contact",
    "suppressed",
    "quota_exceeded",
]);

// EmailEventTimeline renders the provider webhook history for a direct send's
// email delivery. Fetched separately from everything else because it is unbounded
// — one raw provider event body appended per webhook. See agent-docs/overview.md,
// Phase 9.1.
//
// ⚠️ It does NOT repeat the status tag. The tree one column over already renders
// that outcome; showing it twice made the page look like it was reporting two
// different things. This block's job is the REASON and the timeline.
function EmailEventTimeline({
    delivery,
}: {
    delivery: NotificationDeliveryDetail;
}) {
    const copy = deliveryOutcomeText(
        delivery.status,
        delivery.failure_reason ?? undefined
    );

    return (
        <div className="space-y-3 text-sm">
            {copy && <p className="text-text-primary">{copy.long}</p>}

            {delivery.address_snapshot && (
                <p className="text-text-muted text-xs">
                    Sent to <Mono>{delivery.address_snapshot}</Mono>
                    {delivery.attempt > 1 && ` · attempt ${delivery.attempt}`}
                </p>
            )}

            {delivery.events.length === 0 ? (
                // ⚠️ "No events YET" is only honest when events could still arrive.
                // For an email that never reached the provider there will never be
                // any, and "yet" leaves the reader waiting for something that is
                // not coming.
                <p className="text-text-muted text-xs">
                    {NEVER_SENT_STATUSES.has(delivery.status)
                        ? "This email was never sent, so there are no provider events."
                        : "No provider events recorded yet."}
                </p>
            ) : (
                <ol className="border-border-subtle space-y-2 border-l pl-4">
                    {delivery.events.map((event, i) => (
                        <li key={i} className="relative">
                            <span className="bg-text-muted absolute -left-[1.3rem] top-[0.45rem] size-1.5 rounded-full" />
                            <span className="text-text-primary">
                                {EVENT_KIND_LABEL[event.kind] ?? event.kind}
                            </span>
                            {event.at && (
                                <span className="text-text-muted ml-2 text-xs">
                                    {formatDate(new Date(event.at), {
                                        time: true,
                                    })}
                                </span>
                            )}
                            {(event.kind === "opened" ||
                                event.kind === "clicked") && (
                                <Tooltip content={OPEN_SOFT_SIGNAL_COPY}>
                                    <span className="text-text-muted ml-2 cursor-help text-xs underline decoration-dotted underline-offset-2">
                                        soft signal
                                    </span>
                                </Tooltip>
                            )}
                        </li>
                    ))}
                </ol>
            )}
        </div>
    );
}

function DetailShell({
    title,
    target,
    status,
    facts,
    payload,
    tree,
    treeLoading,
    treeError,
    extra,
    backKind,
}: {
    title: string;
    target: string;
    status: ReactNode;
    facts: Fact[];
    payload: unknown;
    tree?: DeliveryTree;
    treeLoading: boolean;
    treeError: boolean;
    extra?: { label: string; body: ReactNode };
    /** Which list tab the breadcrumb returns to. */
    backKind: "direct" | "broadcast";
}) {
    const projectID = useGetProjectIDFromParams();
    const verdict = tree ? verdictFor(tree) : undefined;

    return (
        <div className="mx-auto max-w-6xl px-1 pb-16">
            {/* Breadcrumb, not a back button: it names where you are, and it is the
                only navigation this page needs. */}
            <Link
                to="/projects/$id/notifications"
                params={{ id: projectID }}
                search={{ kind: backKind }}
                className="text-text-muted hover:text-text-primary flex-x mb-5 w-fit items-center gap-1.5 text-sm"
            >
                <IconArrowLeft size={14} />
                Notifications
            </Link>

            <header className="mb-6 flex flex-wrap items-baseline gap-x-4 gap-y-2">
                <h1 className="text-text-primary text-xl font-medium">
                    {title}
                </h1>
                <Mono>
                    <span className="text-text-muted">{target}</span>
                </Mono>
                {status}
            </header>

            {verdict && (
                <div className="border-border-subtle mb-8 border-y py-5">
                    <VerdictBand verdict={verdict} />
                </div>
            )}

            <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_260px]">
                <div className="min-w-0 space-y-8">
                    <section>
                        <Eyebrow>Fan-out</Eyebrow>
                        {treeLoading && <Loading />}
                        {treeError && (
                            <ErrorMessage errorMsg="Could not load the delivery breakdown." />
                        )}
                        {tree && <DeliveryTreeView tree={tree} />}
                    </section>

                    {extra && (
                        <section>
                            <Eyebrow>{extra.label}</Eyebrow>
                            {extra.body}
                        </section>
                    )}

                    <section>
                        <Eyebrow>Payload</Eyebrow>
                        <PayloadBlock payload={payload} />
                    </section>
                </div>

                <aside>
                    <Eyebrow>Details</Eyebrow>
                    <FactList facts={facts} />
                </aside>
            </div>
        </div>
    );
}

export function DirectNotificationDetail({
    notificationID,
}: {
    notificationID: string;
}) {
    const projectID = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useNotification(
        projectID,
        notificationID
    );
    const treeQuery = useNotificationDeliveryTree(projectID, notificationID);
    const deliveriesQuery = useNotificationDeliveries(
        projectID,
        Number(notificationID)
    );

    const notification = data?.data as Notification | undefined;

    const facts = useMemo<Fact[]>(() => {
        if (!notification) return [];

        const sent = new Date(notification.created_at);
        const completed = notification.completed_at
            ? new Date(notification.completed_at)
            : null;

        return [
            {
                label: "Recipient",
                value: (
                    <RecipientLink recipientID={notification.recipient_id} />
                ),
            },
            { label: "Sent", value: formatDate(sent, { time: true }) },
            ...(completed
                ? [
                      {
                          label: "Resolved in",
                          value: formatDuration(sent, completed),
                          hint: "Time from the send request to the in-app outcome being resolved by the worker.",
                      },
                  ]
                : []),
            { label: "Read", value: notification.state.read ? "Yes" : "No" },
            {
                label: "Opened",
                value: notification.state.opened ? "Yes" : "No",
                hint: OPEN_SOFT_SIGNAL_COPY,
            },
            ...(notification.broadcast_id
                ? [
                      {
                          label: "From broadcast",
                          value: (
                              <Link
                                  to="/projects/$id/broadcasts/$broadcastId"
                                  params={{
                                      id: projectID,
                                      broadcastId: String(
                                          notification.broadcast_id
                                      ),
                                  }}
                                  className="underline underline-offset-2"
                              >
                                  #{notification.broadcast_id}
                              </Link>
                          ),
                      },
                  ]
                : []),
        ];
    }, [notification, projectID]);

    if (isLoading) return <Loading />;
    if (isError || !notification) {
        return <ErrorMessage errorMsg="Could not load this notification." />;
    }

    // Only ever one email delivery row per notification (UNIQUE on
    // (notification_id, medium)).
    const emailDelivery = deliveriesQuery.data?.data?.deliveries?.find(
        (d) => d.medium === "email"
    );

    return (
        <DetailShell
            backKind="direct"
            title={`Notification #${notification.id}`}
            target={targetToString(notification.target)}
            status={<StatusTag status={notification.status} />}
            facts={facts}
            payload={notification.payload}
            tree={treeQuery.data?.data}
            treeLoading={treeQuery.isLoading}
            treeError={treeQuery.isError}
            extra={
                emailDelivery
                    ? {
                          label: "Email",
                          body: (
                              <Card>
                                  <EmailEventTimeline
                                      delivery={emailDelivery}
                                  />
                              </Card>
                          ),
                      }
                    : undefined
            }
        />
    );
}

export function BroadcastNotificationDetail({
    broadcastID,
}: {
    broadcastID: string;
}) {
    const projectID = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useBroadcast(projectID, broadcastID);
    const treeQuery = useBroadcastDeliveryTree(projectID, Number(broadcastID));

    const broadcast = data?.data as Broadcast | undefined;

    const facts = useMemo<Fact[]>(() => {
        if (!broadcast) return [];

        const sent = new Date(broadcast.created_at);
        const completed = broadcast.completed_at
            ? new Date(broadcast.completed_at)
            : null;

        return [
            { label: "Sent", value: formatDate(sent, { time: true }) },
            completed
                ? {
                      label: "Fanned out in",
                      value: formatDuration(sent, completed),
                      hint: "Time from the send request to the last batch completing.",
                  }
                : {
                      label: "Fanned out in",
                      value: (
                          <span className="text-text-muted">
                              still running
                          </span>
                      ),
                  },
        ];
    }, [broadcast]);

    if (isLoading) return <Loading />;
    if (isError || !broadcast) {
        return <ErrorMessage errorMsg="Could not load this broadcast." />;
    }

    return (
        <DetailShell
            backKind="broadcast"
            title={`Broadcast #${broadcast.id}`}
            target={targetToString(broadcast.target)}
            status={<StatusTag status={broadcast.status} />}
            facts={facts}
            payload={broadcast.payload}
            tree={treeQuery.data?.data}
            treeLoading={treeQuery.isLoading}
            treeError={treeQuery.isError}
        />
    );
}
