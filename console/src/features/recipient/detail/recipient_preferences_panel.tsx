import { useMemo, useState } from "react";
import {
    ErrorMessage,
    IconInfo,
    Loading,
    Switch,
    Tag,
    toast,
    Tooltip,
} from "netra";

import {
    useGetRecipientPreferences,
    useUpsertRecipientPreference,
} from "@/features/preference/preference_hooks";
import {
    PREFERENCE_MEDIUMS,
    PREFERENCE_MEDIUM_LABELS,
    PreferenceMedium,
    PreferenceSource,
    RecipientPreferenceTargetState,
} from "@/features/preference/preference_type";
import { Target } from "@/features/notification/notification_types";
import { targetToString } from "@/lib/utils";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";
import { apiErrorHandler } from "@/lib/api";

interface RecipientPreferencesPanelProps {
    projectID: string;
    recipientID: string;
}

/** One target's row: the shared target identity plus a cell per medium. */
interface PreferenceRow {
    key: string;
    target: Target;
    name?: string;
    cells: Partial<Record<PreferenceMedium, RecipientPreferenceTargetState>>;
}

/**
 * Explains WHERE a cell's value came from. The value itself is on the switch;
 * this says which rung of the cascade decided it, which is the difference
 * between "we set this" and "this is just the default".
 */
function sourceCopy(
    source: PreferenceSource,
    medium: PreferenceMedium,
    target: Target
): string {
    const anyRule = `${target.channel}/any/${target.event}`;

    switch (source) {
        case "recipient_exact":
            return "Set for this recipient.";
        case "recipient_any":
            return `From this recipient's own ${anyRule} rule, which covers every topic in ${target.channel}.`;
        case "project_exact":
            return "Following the project default for this target.";
        case "project_any":
            return `Following the project's ${anyRule} default, which covers every topic in ${target.channel}.`;
        case "default":
            return medium === "in_app"
                ? "No rule anywhere, so the in-app default applies: in-app delivers unless something mutes it."
                : "No rule anywhere, so the email default applies: email stays off until the target is cataloged for email or this recipient turns it on.";
    }
}

/**
 * The editable per-(target, medium) preference grid — the surface Phase 2
 * deferred for want of a recipient detail page.
 *
 * Every cell shows the RESOLVED decision: what a send would actually do,
 * computed server-side by the same cascade the send path gates on. That is the
 * whole point. Notably `cataloged` is shown as context but never as a gate —
 * the catalog is a default, so an uncataloged target with an explicit recipient
 * row still delivers, and labelling it "unavailable" would be a lie.
 *
 * A toggle writes ONE (target, medium) through the existing console PUT, which
 * converges at the repository layer with the Developer API's PATCH and the
 * email one-click unsubscribe. That is why unsubscribing and this toggle stay
 * in sync.
 */
export function RecipientPreferencesPanel({
    projectID,
    recipientID,
}: RecipientPreferencesPanelProps) {
    const { data, isLoading, isError } = useGetRecipientPreferences(
        projectID,
        recipientID
    );

    const preferences = useMemo(() => data?.data?.preferences ?? [], [data]);

    // Group the flat (target, medium) cells the API returns into one row per
    // target. Grouping is presentation only — every resolved value stays
    // exactly as the server computed it.
    const rows = useMemo(() => {
        const byTarget = new Map<string, PreferenceRow>();

        for (const pref of preferences) {
            const target: Target = {
                channel: pref.target.channel,
                topic: pref.target.topic,
                event: pref.target.event,
            };
            const key = targetToString(target);

            let row = byTarget.get(key);
            if (!row) {
                row = { key, target, cells: {} };
                byTarget.set(key, row);
            }

            row.cells[pref.target.medium] = pref;
            // Only cataloged cells carry a name, and a target may be cataloged
            // for one medium and not the other — take whichever has it.
            if (pref.target.name) {
                row.name = pref.target.name;
            }
        }

        return [...byTarget.values()];
    }, [preferences]);

    if (isError) {
        return <ErrorMessage errorMsg="Error loading preferences" />;
    }

    if (isLoading) {
        return <Loading />;
    }

    if (rows.length === 0) {
        return (
            <p className="text-foreground-muted text-sm max-w-2xl">
                This project has no preference catalog yet, and this recipient
                has no preferences of their own, so there is nothing to show.
                Create a project preference to define what can be subscribed to.
            </p>
        );
    }

    return (
        <div className="space-y-4">
            <p className="text-foreground-muted text-sm max-w-2xl">
                What this recipient would receive right now, per target and
                medium. Each switch is the <strong>resolved</strong> answer —
                the same decision a send makes — not just a stored setting.
                Turning one on or off saves a preference for this recipient
                immediately.
            </p>

            <div className="overflow-x-auto">
                <table className="w-full text-sm">
                    <thead>
                        <tr className="border-border border-b text-left">
                            <th className="py-2 pr-4 font-normal">
                                <TargetInfoTooltip>
                                    <span className="flex-x w-fit">
                                        Target <IconInfo />
                                    </span>
                                </TargetInfoTooltip>
                            </th>
                            {PREFERENCE_MEDIUMS.map((medium) => (
                                <th
                                    key={medium}
                                    className="w-40 py-2 pr-4 font-normal"
                                >
                                    {PREFERENCE_MEDIUM_LABELS[medium]}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row) => (
                            <tr
                                key={row.key}
                                className="border-border/50 border-b align-top"
                            >
                                <td className="py-3 pr-4">
                                    <div className="select-text!">
                                        {row.key}
                                    </div>
                                    {row.name && (
                                        <div className="text-foreground-muted text-xs">
                                            {row.name}
                                        </div>
                                    )}
                                </td>

                                {PREFERENCE_MEDIUMS.map((medium) => (
                                    <td key={medium} className="py-3 pr-4">
                                        <PreferenceCell
                                            projectID={projectID}
                                            recipientID={recipientID}
                                            target={row.target}
                                            medium={medium}
                                            cell={row.cells[medium]}
                                        />
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}

function PreferenceCell({
    projectID,
    recipientID,
    target,
    medium,
    cell,
}: {
    projectID: string;
    recipientID: string;
    target: Target;
    medium: PreferenceMedium;
    cell?: RecipientPreferenceTargetState;
}) {
    // Optimistic switch position, held until the refetched read lands (the
    // mutation's invalidation is awaited). The clicked cell always settles on
    // the clicked value — a recipient-exact row outranks every other rung — but
    // the write can move OTHER cells, which is why the whole grid re-reads
    // rather than patching this one in place.
    const [pending, setPending] = useState<boolean | null>(null);

    const { mutate, isPending } = useUpsertRecipientPreference(
        projectID,
        recipientID,
        {
            onSuccess: () => setPending(null),
            onError: (err: unknown) => {
                setPending(null);
                apiErrorHandler(err);
            },
        }
    );

    // The API resolves every (target, medium) pair it knows a target for, so a
    // missing cell means the medium list and the response disagree. Say so
    // rather than rendering a switch over nothing.
    if (!cell) {
        return <span className="text-foreground-muted">—</span>;
    }

    const enabled = pending ?? cell.state.enabled;

    const onChange = (next: boolean) => {
        setPending(next);
        mutate(
            {
                channel: target.channel,
                topic: target.topic,
                event: target.event,
                medium,
                enabled: next,
            },
            {
                onSuccess: () => {
                    toast.success(
                        `${PREFERENCE_MEDIUM_LABELS[medium]} ${
                            next ? "enabled" : "disabled"
                        } for ${targetToString(target)}`
                    );
                },
            }
        );
    };

    return (
        // The switch sits on its own line with the status chips grouped
        // beneath it, rather than the first chip riding alongside the toggle —
        // that put "Inherited" level with the switch but wrapped "Not cataloged"
        // to its own line, so a two-chip cell read as ragged.
        <div className="space-y-1.5">
            <Switch
                checked={enabled}
                disabled={isPending}
                onCheckedChange={onChange}
                aria-label={`${PREFERENCE_MEDIUM_LABELS[medium]} for ${targetToString(
                    target
                )}`}
            />

            <div className="flex flex-wrap items-center gap-1">
                <Tooltip
                    content={sourceCopy(cell.state.source, medium, target)}
                >
                    <span className="flex-x w-fit">
                        {cell.state.inherited ? (
                            <Tag variant="muted" size="small">
                                Inherited
                            </Tag>
                        ) : (
                            <Tag variant="default" size="small">
                                Set
                            </Tag>
                        )}
                    </span>
                </Tooltip>

                {!cell.state.cataloged && (
                    <Tooltip
                        content={
                            medium === "in_app"
                                ? "This target is not in the project catalog for in-app. In-app delivers by default, so it still sends — the catalog is a default, not a gate."
                                : "This target is not in the project catalog for email. Email is off by default, but an explicit preference here still overrides that and sends."
                        }
                    >
                        <span className="flex-x w-fit">
                            <Tag variant="muted" size="small">
                                Not cataloged
                            </Tag>
                            <IconInfo size={12} />
                        </span>
                    </Tooltip>
                )}
            </div>
        </div>
    );
}
