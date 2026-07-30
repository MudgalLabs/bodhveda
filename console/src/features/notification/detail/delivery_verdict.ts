import {
    DeliveryOutcome,
    DeliveryTree,
} from "@/features/notification/notification_types";

// The verdict: this page's answer, in a sentence.
//
// The detail page used to present fields and leave the reader to compute the
// outcome from a tree — read five nested counts, do the arithmetic, infer whether
// anything is wrong. The tree is evidence; this is the conclusion. It is the one
// thing an operator opening this page actually wants, so it leads.
//
// ⚠️ A broadcast and a direct send get DIFFERENT verdict shapes on purpose. "20 of
// 25 recipients reached" is the right frame for a fan-out; for a fan-out of one it
// would always read "1 of 1", which says nothing. A direct send's real question is
// per-medium ("in-app landed, email didn't"), so that is what it states.

export type VerdictTone = "success" | "warning" | "error" | "neutral";

export interface Verdict {
    tone: VerdictTone;
    /** Large, scannable. A ratio for broadcasts; a status phrase for direct. */
    headline: string;
    /** What the headline is counting. Empty for direct sends. */
    unit?: string;
    /** The why, when there is one worth stating. Plain language, actionable. */
    detail?: string;
}

const MEDIUM_NAME: Record<string, string> = {
    in_app: "In-app",
    email: "Email",
    sms: "SMS",
    web_push: "Web push",
    mobile_push: "Mobile push",
};

function dominantOutcome(outcomes: Record<string, number>): DeliveryOutcome {
    // Order matters: a failure anywhere is the headline, then anything still in
    // flight, then a deliberate suppression, then success.
    const priority: DeliveryOutcome[] = [
        "failed",
        "pending",
        "suppressed",
        "not_requested",
        "succeeded",
    ];
    return priority.find((o) => (outcomes[o] ?? 0) > 0) ?? "succeeded";
}

const TONE_FOR_OUTCOME: Record<DeliveryOutcome, VerdictTone> = {
    failed: "error",
    pending: "warning",
    // ⚠️ Suppressed is NOT an error tone. The recipient opted out; the system did
    // its job. Colouring it red would make a healthy send look broken.
    suppressed: "neutral",
    not_requested: "neutral",
    succeeded: "success",
};

export function verdictFor(tree: DeliveryTree): Verdict {
    return tree.kind === "broadcast"
        ? broadcastVerdict(tree)
        : directVerdict(tree);
}

function broadcastVerdict(tree: DeliveryTree): Verdict {
    const audience = tree.audience;
    const inApp = tree.mediums.find((m) => m.medium === "in_app");

    const reached = inApp?.outcomes.succeeded ?? 0;
    const pending = inApp?.pending ?? 0;
    const failed = inApp?.outcomes.failed ?? 0;

    // No audience recorded is its own answer — and NOT the same as reaching
    // nobody, which is why it gets its own branch rather than a zeroed ratio.
    if (!audience) {
        return {
            tone: "neutral",
            headline: "Audience not recorded",
            detail: "This broadcast was sent before audience counts were recorded, or its fan-out has not run yet.",
        };
    }

    const total = audience.total;

    if (audience.eligible === 0) {
        // The most useful thing this page can say. Name the cause, because the two
        // reasons need opposite fixes.
        const notCataloged = audience.excluded_not_cataloged > 0;
        return {
            tone: "warning",
            headline: `0 of ${total}`,
            unit: total === 1 ? "recipient reached" : "recipients reached",
            detail: notCataloged
                ? "This target has no enabled in-app catalog entry, so no one could receive it. Add or enable it in Preferences."
                : "Every recipient has this target turned off. Nothing to fix — this is their choice.",
        };
    }

    const parts: string[] = [];
    if (audience.excluded_disabled > 0) {
        parts.push(`${audience.excluded_disabled} opted out`);
    }
    if (audience.excluded_not_cataloged > 0) {
        parts.push(`${audience.excluded_not_cataloged} not cataloged`);
    }
    if (pending > 0) parts.push(`${pending} still pending`);
    if (failed > 0) parts.push(`${failed} failed`);

    let tone: VerdictTone = "success";
    if (failed > 0) tone = "error";
    else if (pending > 0) tone = "warning";

    return {
        tone,
        headline: `${reached} of ${total}`,
        unit: total === 1 ? "recipient reached" : "recipients reached",
        detail: parts.length ? parts.join(" · ") : undefined,
    };
}

function directVerdict(tree: DeliveryTree): Verdict {
    // Per-medium, because a ratio over one recipient tells you nothing.
    const phrases = tree.mediums.map((m) => {
        const outcome = dominantOutcome(m.outcomes);
        const name = MEDIUM_NAME[m.medium] ?? m.medium;

        switch (outcome) {
            case "succeeded":
                return { text: `${name} delivered`, outcome };
            case "failed":
                return { text: `${name} failed`, outcome };
            case "pending":
                return { text: `${name} pending`, outcome };
            case "suppressed":
                return { text: `${name} suppressed`, outcome };
            default:
                return { text: `${name} not requested`, outcome };
        }
    });

    const worst = dominantOutcome(
        Object.fromEntries(
            phrases.map((p) => [p.outcome, 1])
        ) as Record<string, number>
    );

    return {
        tone: TONE_FOR_OUTCOME[worst],
        headline: phrases.map((p) => p.text).join(" · "),
    };
}
