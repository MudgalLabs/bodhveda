import { DeliveryStatus } from "@/features/notification/notification_types";

// Human copy for the email medium's delivery outcomes.
//
// `failure_reason` is a backend slug (service/notification.go's fanOutEmail and
// the email:delivery worker). It exists for ONE reason above all: `muted` has two
// completely different causes that a operator must be able to tell apart —
// `not_cataloged` (the PROJECT never offered email on this target) vs
// `preference_disabled` (the RECIPIENT opted out). Same status, opposite fix.
// Rendering the raw slug would push that distinction back onto the reader, so
// every reason gets real prose here.
//
// The wording deliberately matches the post-send toast (notifyEmailOutcome in
// send_notification_modal.tsx) — the same skip should read the same way whether
// you see it right after sending or a week later in the list.

interface OutcomeCopy {
    /** Terse phrase for the space-constrained list line. */
    short: string;
    /** Full explanation — what happened and what to do about it. */
    long: string;
}

const REASON_COPY: Record<string, OutcomeCopy> = {
    not_cataloged: {
        short: "target not cataloged for email",
        long: "This target has no project-level email catalog entry, so email can never fire for it. Add email to the target in Preferences to enable it.",
    },
    preference_disabled: {
        short: "recipient opted out",
        long: "The recipient has email turned off for this target — either from your app's settings or via the one-click unsubscribe in a previous email.",
    },
    provider_not_configured: {
        short: "no email provider configured",
        long: "This project has no email provider settings, so nothing could send. Add your Resend API key and from-identity in Settings → Email.",
    },
    provider_send_error: {
        short: "provider rejected the send",
        long: "The email provider returned an error when accepting this message. The raw provider response is in the event history below.",
    },
    provider_lookup_error: {
        short: "provider settings unreadable",
        long: "The project's email settings could not be loaded when this send was resolved. This is a Bodhveda-side error, not a provider rejection.",
    },
    contact_lookup_error: {
        short: "contact lookup failed",
        long: "The recipient's primary email contact could not be looked up when this send was resolved. This is a Bodhveda-side error.",
    },
    gating_error: {
        short: "preference check failed",
        long: "The catalog and preference gate could not be evaluated for this send, so email was not attempted. This is a Bodhveda-side error.",
    },
    secret_decrypt_error: {
        short: "provider key unreadable",
        long: "The stored provider API key could not be decrypted. Re-enter the key in Settings → Email.",
    },
    adapter_init_error: {
        short: "provider adapter failed",
        long: "The email provider adapter could not be constructed from this project's settings. Check the configured provider in Settings → Email.",
    },
};

/**
 * Human copy for a delivery's failure_reason. Returns null when there is no
 * reason to explain. An unrecognized slug degrades to a readable form of itself
 * rather than being dropped — a new backend reason should still say something.
 */
export function deliveryFailureReasonText(reason?: string): OutcomeCopy | null {
    if (!reason) return null;

    const known = REASON_COPY[reason];
    if (known) return known;

    const humanized = reason.replace(/_/g, " ");
    return { short: humanized, long: humanized };
}

/**
 * Explains a delivery status that carries no failure_reason but still isn't a
 * plain success — today only `no_contact`, which fanOutEmail records with no
 * reason because the status already says it.
 */
export function deliveryStatusText(status: DeliveryStatus): OutcomeCopy | null {
    if (status === "no_contact") {
        return {
            short: "no email address on file",
            long: "The recipient has no primary email contact, so there was nowhere to send. Add an email contact for this recipient.",
        };
    }
    return null;
}

/**
 * The single explanation for an email delivery outcome, if it needs one.
 * failure_reason wins when present; otherwise the status may speak for itself.
 */
export function deliveryOutcomeText(
    status: DeliveryStatus,
    failureReason?: string
): OutcomeCopy | null {
    return deliveryFailureReasonText(failureReason) ?? deliveryStatusText(status);
}

// The soft-signal caveat for email opens — the single source of that wording, so
// the console tells one consistent story wherever an open count appears (the
// delivery detail dialog, the Home dashboard's Email panel): email "opened" is
// directional only, while in-app "read" is trustworthy.
export const OPEN_SOFT_SIGNAL_COPY =
    "A soft, directional signal only — open tracking is unreliable (e.g. Apple Mail Privacy Protection pre-fetches images). In-app “read” is the trustworthy signal.";

// Labels for the normalized provider webhook event kinds (backend
// email.WebhookEventKind). An unknown kind falls back to the raw event.
export const EVENT_KIND_LABEL: Record<string, string> = {
    sent: "Accepted by provider",
    delivered: "Delivered to mail server",
    bounced: "Bounced",
    complained: "Marked as spam",
    opened: "Opened",
    clicked: "Link clicked",
};
