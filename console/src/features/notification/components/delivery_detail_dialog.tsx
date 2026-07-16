import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    ErrorMessage,
    IconInfo,
    Loading,
    Separator,
    Tooltip,
    formatDate,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useNotificationDeliveries } from "@/features/notification/notification_hooks";
import {
    DeliveryEvent,
    Notification,
    NotificationDeliveryDetail,
} from "@/features/notification/notification_types";
import {
    EVENT_KIND_LABEL,
    OPEN_SOFT_SIGNAL_COPY,
    deliveryOutcomeText,
} from "@/features/notification/delivery_copy";
import { StatusTag } from "@/components/status_tag";
import { targetToString } from "@/lib/utils";

interface DeliveryDetailDialogProps {
    notification: Notification;
    open: boolean;
    setOpen: (open: boolean) => void;
}

// DeliveryDetailDialog shows how one notification actually landed, per medium.
//
// It is NOTIFICATION-scoped, not email-scoped: the in-app outcome is rendered
// from the notification row itself (in_app has no notification_delivery row —
// Phase 4 deliberately left its state on the notification), while email renders
// from its delivery record. That asymmetry is real, so the dialog names it rather
// than hiding it.
//
// The email event history is fetched HERE rather than on the notifications list,
// since it is unbounded (one raw provider event body appended per webhook) and is
// only ever read one delivery at a time. See agent-docs/overview.md, Phase 9.1.
export function DeliveryDetailDialog({
    notification,
    open,
    setOpen,
}: DeliveryDetailDialogProps) {
    const projectID = useGetProjectIDFromParams();

    // `open` gates the fetch — that gating is the point of the split. The in-app
    // section below needs no fetch at all; it renders from the list row.
    const { data, isLoading, isError } = useNotificationDeliveries(
        projectID,
        notification.id,
        open
    );

    const deliveries = data?.data?.deliveries ?? [];

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            {/* `sm:max-w-3xl!` needs the `!`: netra's stylesheet is imported in
                main.tsx AFTER index.html links the console's Tailwind, so netra's
                precompiled utilities always win on source order at equal
                specificity (media queries add none). Without it, netra's own
                `sm:max-w-lg` is stripped by its cn()'s twMerge in favour of ours,
                ours then loses to netra's base `max-w-[calc(100%-2rem)]`, and the
                dialog renders full-bleed. Same reason as `select-text!` etc.
                elsewhere in the console. */}
            <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl!">
                <DialogHeader>
                    <DialogTitle className="flex-x">
                        Delivery detail
                        {isLoading && <Loading />}
                    </DialogTitle>
                    <DialogDescription>
                        {targetToString(notification.target)} &middot;{" "}
                        {notification.recipient_id}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-6">
                    <InAppSection notification={notification} />

                    <Separator />

                    {isError ? (
                        <ErrorMessage errorMsg="Error loading email delivery" />
                    ) : isLoading ? (
                        <p className="text-text-muted text-sm">
                            Loading email delivery…
                        </p>
                    ) : deliveries.length === 0 ? (
                        <EmptyEmailSection notification={notification} />
                    ) : (
                        deliveries.map((d) => (
                            <DeliverySection key={d.id} delivery={d} />
                        ))
                    )}
                </div>
            </DialogContent>
        </Dialog>
    );
}

// InAppSection renders the in-app outcome from the notification row. There is no
// delivery record to fetch: in v1 `notification_delivery` is written for email
// only, and in-app's status/read/opened live on the notification itself.
//
// The payload is the point of this section — it is the in-app content block as
// sent, and it appears nowhere else in the console.
function InAppSection({ notification }: { notification: Notification }) {
    return (
        <div className="space-y-4">
            <div className="flex-x gap-x-2">
                <span className="text-text-primary text-sm font-medium">
                    In-app
                </span>
                <StatusTag status={notification.status} />
            </div>

            <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Field
                    label="Sent"
                    value={formatTimestamp(notification.created_at)}
                />
                <Field
                    label="Resolved"
                    value={formatTimestamp(notification.completed_at)}
                    hint="When the delivery pipeline finished resolving this notification's in-app outcome."
                />
                <Field
                    label="Read"
                    value={notification.state.read ? "Yes" : "No"}
                    hint="A first-party signal from your own app — unlike email “opened”, this one is trustworthy."
                />
                <Field
                    label="Opened"
                    value={notification.state.opened ? "Yes" : "No"}
                />
            </dl>

            <div>
                <div className="mb-2 flex-x gap-x-2">
                    <h3 className="text-text-primary text-sm font-medium">
                        Payload
                    </h3>
                    <span className="text-text-muted text-xs">
                        the in-app content block, as sent
                    </span>
                </div>
                <pre className="border-border-subtle bg-surface-1 text-text-muted overflow-x-auto rounded-md border p-2 text-xs">
                    {formatPayload(notification.payload)}
                </pre>
            </div>
        </div>
    );
}

// EmptyEmailSection explains WHY there is no email delivery record. The two
// causes are different facts and must not read the same: the send carried no
// email block at all, vs. the row is unexpectedly missing.
function EmptyEmailSection({ notification }: { notification: Notification }) {
    return (
        <div className="space-y-2">
            <span className="text-text-primary text-sm font-medium">Email</span>
            <p className="text-text-muted text-sm">
                {notification.email
                    ? "No delivery record found for this notification’s email."
                    : "No email was included in this send, so no email was attempted. Email fires only when the send carries an email block."}
            </p>
        </div>
    );
}

function DeliverySection({
    delivery,
}: {
    delivery: NotificationDeliveryDetail;
}) {
    const outcome = deliveryOutcomeText(
        delivery.status,
        delivery.failure_reason
    );

    return (
        <div className="space-y-4">
            <div className="flex-x gap-x-2">
                <span className="text-text-primary text-sm font-medium capitalize">
                    {delivery.medium === "email" ? "Email" : delivery.medium}
                </span>
                <StatusTag status={delivery.status} />
            </div>

            {outcome && (
                <p className="border-border-subtle bg-surface-1 text-text-muted rounded-md border p-3 text-sm">
                    {outcome.long}
                </p>
            )}

            <FieldGrid delivery={delivery} />

            <div>
                <div className="mb-2 flex-x gap-x-2">
                    <h3 className="text-text-primary text-sm font-medium">
                        Event history
                    </h3>
                    <span className="text-text-muted text-xs">
                        from the provider’s webhooks
                    </span>
                </div>
                <EventTimeline
                    events={delivery.events}
                    status={delivery.status}
                />
            </div>
        </div>
    );
}

function FieldGrid({ delivery }: { delivery: NotificationDeliveryDetail }) {
    return (
        <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field
                label="Sent to"
                value={delivery.address_snapshot}
                hint="The address captured when this send was enqueued — later edits to the recipient’s contact don’t change it."
            />
            <Field
                label="Attempts"
                value={delivery.attempt > 0 ? String(delivery.attempt) : "—"}
            />
            <Field label="Provider" value={delivery.provider} />
            <Field
                label="Provider message ID"
                value={delivery.provider_message_id}
                hint="How the provider’s webhooks correlate back to this delivery."
                mono
            />

            <div className="sm:col-span-2">
                <Separator />
            </div>

            <Field
                label="Resolved"
                value={formatTimestamp(delivery.created_at)}
            />
            <Field label="Sent" value={formatTimestamp(delivery.sent_at)} />
            <Field
                label="Delivered"
                value={formatTimestamp(delivery.delivered_at)}
            />
            <Field
                label="Opened"
                value={formatTimestamp(delivery.opened_at)}
                hint={OPEN_SOFT_SIGNAL_COPY}
            />
            <Field
                label="Clicked"
                value={formatTimestamp(delivery.clicked_at)}
                hint={OPEN_SOFT_SIGNAL_COPY}
            />
            <Field
                label="Bounced"
                value={formatTimestamp(delivery.bounced_at)}
            />
            <Field
                label="Complained"
                value={formatTimestamp(delivery.complained_at)}
            />
        </dl>
    );
}

function Field({
    label,
    value,
    hint,
    mono,
}: {
    label: string;
    value?: string | null;
    hint?: string;
    mono?: boolean;
}) {
    return (
        <div className="flex flex-col gap-y-1">
            <dt className="flex-x text-text-muted text-xs">
                {label}
                {hint && (
                    <Tooltip content={hint}>
                        <IconInfo size={12} />
                    </Tooltip>
                )}
            </dt>
            <dd
                className={`text-text-primary text-sm break-all ${
                    mono ? "font-mono text-xs" : ""
                } ${!value ? "text-text-muted" : ""}`}
            >
                {value || "—"}
            </dd>
        </div>
    );
}

// EventTimeline renders provider_response — a JSONB ARRAY appended once per
// inbound webhook (Phase 5) — as a readable timeline. `kind` and `at` are
// normalized server-side by the provider's adapter; the raw event stays
// available underneath for the cases where only the provider's own words will do.
function EventTimeline({
    events,
    status,
}: {
    events: DeliveryEvent[];
    status: NotificationDeliveryDetail["status"];
}) {
    if (events.length === 0) {
        // Distinguish "waiting on webhooks" from "webhooks were never going to
        // come" — a muted/no_contact email never reached a provider at all.
        const neverSent =
            status === "muted" ||
            status === "no_contact" ||
            status === "failed";

        return (
            <p className="text-text-muted text-sm">
                {neverSent
                    ? "No provider events — this email never reached the provider."
                    : "No provider events yet. Delivery, bounce and open events appear here as the provider reports them."}
            </p>
        );
    }

    return (
        <ol className="space-y-3">
            {events.map((event, i) => (
                <li key={i} className="flex gap-x-3">
                    <div className="flex flex-col items-center pt-1">
                        <span className="bg-text-muted size-2 shrink-0 rounded-full" />
                        {i < events.length - 1 && (
                            <span className="bg-border-subtle mt-1 w-px grow" />
                        )}
                    </div>

                    <div className="min-w-0 grow pb-1">
                        <div className="flex-x gap-x-2">
                            <span className="text-text-primary text-sm">
                                {EVENT_KIND_LABEL[event.kind] ??
                                    "Provider event"}
                            </span>
                            {(event.kind === "opened" ||
                                event.kind === "clicked") && (
                                <Tooltip content={OPEN_SOFT_SIGNAL_COPY}>
                                    <IconInfo size={12} />
                                </Tooltip>
                            )}
                        </div>

                        {event.at && (
                            <span className="text-text-muted text-xs">
                                {formatTimestamp(event.at)}
                            </span>
                        )}

                        <details className="mt-1">
                            <summary className="text-text-muted cursor-pointer text-xs select-none">
                                Raw event
                            </summary>
                            <pre className="border-border-subtle bg-surface-1 text-text-muted mt-1 overflow-x-auto rounded-md border p-2 text-xs">
                                {JSON.stringify(event.raw, null, 2)}
                            </pre>
                        </details>
                    </div>
                </li>
            ))}
        </ol>
    );
}

function formatTimestamp(ts?: string | null): string | null {
    if (!ts) return null;
    return formatDate(new Date(ts), { time: true });
}

// The payload is arbitrary customer JSON, so it is pretty-printed rather than
// interpreted. Anything unstringifiable (e.g. a cycle — not possible over the
// wire, but cheap to be safe) degrades to its raw form instead of throwing
// inside a dialog.
function formatPayload(payload: unknown): string {
    if (payload === null || payload === undefined) return "—";
    try {
        return JSON.stringify(payload, null, 2);
    } catch {
        return String(payload);
    }
}
