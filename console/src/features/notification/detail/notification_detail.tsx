import {
    ErrorMessage,
    IconBell,
    Loading,
    PageHeading,
    Separator,
    Tooltip,
    formatDate,
} from "netra";
import { Link } from "@tanstack/react-router";
import { ReactNode, useMemo } from "react";

import { DeliveryTreeView } from "@/features/notification/components/delivery_tree";
import { StatusTag } from "@/components/status_tag";
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
// This whole surface exists for debugging ("here is the one that failed"), so a
// stable URL is the point, not a nicety. The modals stay for triage-in-place —
// peeking at rows while scanning a filtered list is a real workflow, and
// navigating away loses your filters and scroll position.
//
// ⚠️ Direct and broadcast get SEPARATE routes (/notifications/:id and
// /broadcasts/:id) rather than one route with a kind discriminator, because
// notification.id and broadcast.id are independent SERIAL sequences whose ranges
// overlap — measured 2026-07-30: notifications 1–750563, broadcasts 23–31. One
// path could not tell /notifications/25 apart. They still share this component,
// so the two never drift into unrelated screens.

interface Fact {
    label: string;
    value: ReactNode;
    hint?: string;
}

function FactStrip({ facts }: { facts: Fact[] }) {
    return (
        <div className="border-border-subtle mb-4 flex flex-wrap items-center gap-x-6 gap-y-2 border-b pb-3 text-sm">
            {facts.map(({ label, value, hint }) => {
                const body = (
                    <div className="flex flex-col">
                        <span className="text-text-muted text-xs">{label}</span>
                        <span className="text-text-primary">{value}</span>
                    </div>
                );
                return hint ? (
                    <Tooltip key={label} content={hint}>
                        <span className="cursor-help">{body}</span>
                    </Tooltip>
                ) : (
                    <span key={label}>{body}</span>
                );
            })}
        </div>
    );
}

function Section({
    title,
    children,
}: {
    title: string;
    children: ReactNode;
}) {
    return (
        <section className="mb-6">
            <h2 className="text-text-primary mb-2 text-sm font-medium">
                {title}
            </h2>
            {children}
        </section>
    );
}

function PayloadBlock({ payload }: { payload: unknown }) {
    // A payload-less send is normal (email-only), so say so rather than showing
    // an empty box that reads like a loading state.
    if (payload === null || payload === undefined) {
        return (
            <p className="text-text-muted text-sm">
                No in-app payload — this send did not request in-app delivery.
            </p>
        );
    }

    return (
        <pre className="bg-surface-bg text-text-primary select-text! max-h-64 overflow-auto rounded-md p-3 text-xs">
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

// EmailEventTimeline renders the raw provider webhook history for a direct
// send's email delivery. Fetched separately from everything else because it is
// unbounded — one raw provider event body appended per webhook. See
// agent-docs/overview.md, Phase 9.1.
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
            <div className="flex-x flex-wrap items-center gap-2">
                <StatusTag status={delivery.status} />
                {delivery.address_snapshot && (
                    <span className="text-text-muted select-text! text-xs">
                        {delivery.address_snapshot}
                    </span>
                )}
                {delivery.attempt > 1 && (
                    <span className="text-text-muted text-xs">
                        attempt {delivery.attempt}
                    </span>
                )}
            </div>

            {/* The prose behind the status slug. `muted` in particular has two
                opposite causes — the project never cataloged email, or the
                recipient opted out — and only this copy separates them. */}
            {copy && <p className="text-text-muted">{copy.long}</p>}

            {delivery.events.length === 0 ? (
                // ⚠️ "No events YET" is only honest when events could still
                // arrive. For an email that never reached the provider — muted,
                // no contact, over quota — there will never be any, and implying
                // otherwise leaves the reader waiting for something that is not
                // coming. Same trap as a resolved alert echoing the broken-state
                // copy: technically-empty is not the same as pending.
                <p className="text-text-muted text-xs">
                    {NEVER_SENT_STATUSES.has(delivery.status)
                        ? "This email was never sent, so there are no provider events."
                        : "No provider events recorded yet."}
                </p>
            ) : (
                <ol className="space-y-1">
                    {delivery.events.map((event, i) => (
                        <li key={i} className="flex-x items-baseline gap-2">
                            <span className="text-text-primary">
                                {EVENT_KIND_LABEL[event.kind] ?? event.kind}
                            </span>
                            {event.at && (
                                <span className="text-text-muted text-xs">
                                    {formatDate(new Date(event.at), {
                                        time: true,
                                    })}
                                </span>
                            )}
                            {(event.kind === "opened" ||
                                event.kind === "clicked") && (
                                <Tooltip content={OPEN_SOFT_SIGNAL_COPY}>
                                    <span className="text-text-muted cursor-help text-xs underline underline-offset-2">
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

// DetailShell is everything the two kinds share, so a direct send and a
// broadcast are read the same way.
function DetailShell({
    heading,
    facts,
    payload,
    tree,
    treeLoading,
    treeError,
    extra,
    isFetching,
}: {
    heading: string;
    facts: Fact[];
    payload: unknown;
    tree?: DeliveryTree;
    treeLoading: boolean;
    treeError: boolean;
    extra?: ReactNode;
    isFetching?: boolean;
}) {
    return (
        <div>
            <PageHeading>
                <IconBell size={18} />
                <h1 className="select-text!">{heading}</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <FactStrip facts={facts} />

            <Section title="Delivery">
                {treeLoading && <Loading />}
                {treeError && (
                    <ErrorMessage errorMsg="Error loading delivery breakdown" />
                )}
                {tree && <DeliveryTreeView tree={tree} />}
            </Section>

            {extra}

            <Separator />

            <Section title="Payload">
                <PayloadBlock payload={payload} />
            </Section>
        </div>
    );
}

export function DirectNotificationDetail({
    notificationID,
}: {
    notificationID: string;
}) {
    const projectID = useGetProjectIDFromParams();

    const { data, isLoading, isError, isFetching } = useNotification(
        projectID,
        notificationID
    );
    const treeQuery = useNotificationDeliveryTree(projectID, notificationID);
    // Numeric id for the deliveries endpoint, which predates this page.
    const deliveriesQuery = useNotificationDeliveries(
        projectID,
        Number(notificationID)
    );

    const notification = data?.data as Notification | undefined;

    const facts = useMemo<Fact[]>(() => {
        if (!notification) return [];

        return [
            { label: "Kind", value: "Direct" },
            {
                label: "Target",
                value: (
                    <span className="select-text!">
                        {targetToString(notification.target)}
                    </span>
                ),
            },
            {
                label: "Recipient",
                value: (
                    <RecipientLink recipientID={notification.recipient_id} />
                ),
            },
            {
                label: "In-app status",
                value: <StatusTag status={notification.status} />,
            },
            {
                label: "Sent",
                value: formatDate(new Date(notification.created_at), {
                    time: true,
                }),
            },
            {
                label: "Read",
                value: notification.state.read ? "Yes" : "No",
            },
            {
                label: "Opened",
                value: notification.state.opened ? "Yes" : "No",
                hint: OPEN_SOFT_SIGNAL_COPY,
            },
            ...(notification.broadcast_id
                ? [
                      {
                          label: "Broadcast",
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
        return <ErrorMessage errorMsg="Error loading notification" />;
    }

    // Only ever one email delivery row per notification (UNIQUE on
    // (notification_id, medium)), so the first is the email one.
    const emailDelivery = deliveriesQuery.data?.data?.deliveries?.find(
        (d) => d.medium === "email"
    );

    return (
        <DetailShell
            heading={`Notification #${notification.id}`}
            facts={facts}
            payload={notification.payload}
            tree={treeQuery.data?.data}
            treeLoading={treeQuery.isLoading}
            treeError={treeQuery.isError}
            isFetching={isFetching}
            extra={
                emailDelivery ? (
                    <Section title="Email">
                        <EmailEventTimeline delivery={emailDelivery} />
                    </Section>
                ) : undefined
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

    const { data, isLoading, isError, isFetching } = useBroadcast(
        projectID,
        broadcastID
    );
    const treeQuery = useBroadcastDeliveryTree(projectID, Number(broadcastID));

    const broadcast = data?.data as Broadcast | undefined;

    const facts = useMemo<Fact[]>(() => {
        if (!broadcast) return [];

        return [
            { label: "Kind", value: "Broadcast" },
            {
                label: "Target",
                value: (
                    <span className="select-text!">
                        {targetToString(broadcast.target)}
                    </span>
                ),
            },
            { label: "Status", value: <StatusTag status={broadcast.status} /> },
            {
                label: "Sent",
                value: formatDate(new Date(broadcast.created_at), {
                    time: true,
                }),
            },
            {
                label: "Completed",
                value: broadcast.completed_at ? (
                    formatDate(new Date(broadcast.completed_at), { time: true })
                ) : (
                    <span className="text-text-muted">—</span>
                ),
            },
        ];
    }, [broadcast]);

    if (isLoading) return <Loading />;
    if (isError || !broadcast) {
        return <ErrorMessage errorMsg="Error loading broadcast" />;
    }

    return (
        <DetailShell
            heading={`Broadcast #${broadcast.id}`}
            facts={facts}
            payload={broadcast.payload}
            tree={treeQuery.data?.data}
            treeLoading={treeQuery.isLoading}
            treeError={treeQuery.isError}
            isFetching={isFetching}
        />
    );
}
