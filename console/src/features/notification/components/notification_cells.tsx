import { useState } from "react";
import { formatDuration, IconInfo, Loading, Tooltip } from "netra";

import {
    BroadcastStatus,
    DeliveryStatus,
    Notification,
    NotificationStatus,
} from "@/features/notification/notification_types";
import { DeliveryDetailDialog } from "@/features/notification/components/delivery_detail_dialog";
import { deliveryOutcomeText } from "@/features/notification/delivery_copy";
import { StatusTag } from "@/components/status_tag";

// Shared by the project Notifications list and the recipient detail page's feed
// (Phase 9.2), so a notification reads identically wherever it appears.

// NotificationStatusCell renders the in-app outcome and, when the send carried
// email, the email medium's outcome beneath it — so a diverging result (in-app
// delivered, email muted) is visible per row.
//
// An email that did not deliver states WHY inline: `muted` alone is ambiguous
// between "the project never cataloged email for this target" and "the recipient
// opted out", which are opposite fixes. That distinction is why failure_reason
// exists (Phase 4) and it was previously only legible in the post-send toast.
export function NotificationStatusCell({
    notification,
}: {
    notification: Notification;
}) {
    const createdAt = new Date(notification.created_at);
    const email = notification.email;

    const inAppLine = (
        <MediumStatusLine
            label="In-app"
            status={notification.status}
            elapsed={
                notification.completed_at
                    ? formatDuration(
                          createdAt,
                          new Date(notification.completed_at)
                      )
                    : null
            }
            pending={!notification.completed_at}
        />
    );

    // Email is direct-only, and only present when the send carried an email
    // block.
    if (!email) return inAppLine;

    // Prefer the webhook-confirmed delivery time; fall back to the
    // provider-accepted (sent) time while delivery is unconfirmed.
    const emailTime = email.delivered_at ?? email.sent_at ?? null;
    const outcome = deliveryOutcomeText(email.status, email.failure_reason);

    return (
        <div className="space-y-1">
            {inAppLine}
            <MediumStatusLine
                label="Email"
                status={email.status}
                elapsed={
                    emailTime
                        ? formatDuration(createdAt, new Date(emailTime))
                        : null
                }
                pending={
                    email.status === "pending" || email.status === "sending"
                }
                reason={outcome}
            />
        </div>
    );
}

export function MediumStatusLine({
    label,
    status,
    elapsed,
    pending,
    reason,
}: {
    label: string;
    status: NotificationStatus | BroadcastStatus | DeliveryStatus;
    elapsed: string | null;
    pending?: boolean;
    /** Human copy for a non-delivering outcome; shown inline beside the tag. */
    reason?: { short: string; long: string } | null;
}) {
    return (
        <span className="flex-x gap-x-2">
            <span className="text-xs text-text-muted w-14 shrink-0">
                {label}
            </span>
            <StatusTag status={status} />
            {elapsed ? (
                <span className="text-xs text-text-muted">{elapsed}</span>
            ) : pending ? (
                <Loading size={18} />
            ) : null}

            {/* The short phrase makes the row self-explanatory at a glance; the
                tooltip carries the full explanation and the fix. */}
            {reason && (
                <Tooltip content={reason.long}>
                    <span className="flex-x gap-x-1 text-xs text-text-muted">
                        <IconInfo size={12} />
                        {reason.short}
                    </span>
                </Tooltip>
            )}
        </span>
    );
}

// DeliveryDetailCell is the row-level trigger for the detail dialog. It lives in
// its own column rather than on the Status cell's email line because the dialog
// is NOTIFICATION-scoped (in-app + email), not email-scoped — so every row gets
// one, including the in-app-only sends that are still the common case.
export function DeliveryDetailCell({
    notification,
}: {
    notification: Notification;
}) {
    const [open, setOpen] = useState(false);

    return (
        <>
            <button
                type="button"
                onClick={() => setOpen(true)}
                className="text-text-muted hover:text-text-primary cursor-pointer text-xs underline underline-offset-2"
            >
                Details
            </button>

            <DeliveryDetailDialog
                notification={notification}
                open={open}
                setOpen={setOpen}
            />
        </>
    );
}
