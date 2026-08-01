import { useMemo } from "react";
import {
    Card,
    CardContent,
    ErrorMessage,
    IconTarget,
    Loading,
    PageHeading,
    Switch,
    Tag,
    toast,
} from "netra";

import {
    useGetProject,
    useGetProjectIDFromParams,
    useUpdateProject,
} from "@/features/project/project_hooks";
import { useGetCatalogDrift } from "@/features/preference/preference_hooks";
import {
    PREFERENCE_MEDIUM_LABELS,
    UncatalogedTarget,
} from "@/features/preference/preference_type";

/**
 * The strict-targets setting plus the drift report that justifies flipping it.
 *
 * The two belong on screen together. Strict targets defaults to OFF, so nothing
 * ever forces a project to think about it; the drift list is what turns "some
 * setting I ignored" into "14 sends last week named targets I never defined".
 */
export function StrictTargets() {
    const id = useGetProjectIDFromParams();

    const { data, isLoading, isFetching, isError } = useGetProject(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading project settings" />;
        }

        if (isLoading || !data) {
            return <Loading />;
        }

        return <StrictTargetsForm strictTargets={data.data.strict_targets} />;
    }, [data, isError, isLoading]);

    return (
        <div className="mb-10">
            <PageHeading>
                <IconTarget size={18} />
                <h1>Targeting</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <p className="text-text-muted paragraph mb-6 max-w-2xl">
                A target is only <em>defined</em> once it has a project
                preference — the catalog entry recipients toggle and analytics
                group by. These settings decide what happens when you send a
                target that has no entry.
            </p>

            {content}
        </div>
    );
}

interface StrictTargetsFormProps {
    strictTargets: boolean;
}

function StrictTargetsForm({ strictTargets }: StrictTargetsFormProps) {
    const id = useGetProjectIDFromParams();

    const { mutate, isPending } = useUpdateProject({
        onSuccess: () => {
            toast.success(
                strictTargets
                    ? "Strict targets turned off"
                    : "Strict targets turned on"
            );
        },
    });

    const { data: drift, isLoading: driftLoading } = useGetCatalogDrift(id);

    // The project name has to ride along: the update payload validates `name` as
    // required, so a toggle-only PATCH would be rejected.
    const { data: project } = useGetProject(id);

    const onChange = (next: boolean) => {
        if (!project) return;

        mutate({
            id: Number(id),
            name: project.data.name,
            strict_targets: next,
        });
    };

    const uncataloged = drift?.data;
    const hasDrift = (uncataloged?.total_sends ?? 0) > 0;

    return (
        <div className="space-y-4">
            <Card>
                <CardContent>
                    <div className="flex items-start justify-between gap-6">
                        <div className="max-w-2xl">
                            {/*
                              * Labelled so the OFF state is honest. A bare
                              * "Strict targets?" makes off sound like the
                              * absence of a feature; what off actually means is
                              * "this project will happily send targets it never
                              * defined", and that is the thing someone needs to
                              * be able to read off the screen.
                              */}
                            <h2 className="label-medium mb-1">
                                Strict targets
                            </h2>
                            <p className="text-text-muted paragraph">
                                Reject sends to targets this project hasn't
                                cataloged.
                            </p>

                            <p className="text-text-muted paragraph mt-3">
                                {strictTargets ? (
                                    <>
                                        <strong>On.</strong> A send naming a
                                        target with no matching project
                                        preference is rejected with a{" "}
                                        <code>400</code> and nothing is written
                                        — for the mediums that send asks for.
                                        Typos and missing setup fail loudly, on
                                        the first call.
                                    </>
                                ) : (
                                    <>
                                        <strong>Off.</strong> Sends to targets
                                        you've never cataloged are accepted.
                                        In-app notifications still deliver, but
                                        the target appears on nobody's
                                        preference screen, so recipients cannot
                                        mute it — and a typo'd target looks
                                        exactly like a real one.
                                    </>
                                )}
                            </p>
                        </div>

                        <Switch
                            checked={strictTargets}
                            disabled={isPending || !project}
                            onCheckedChange={onChange}
                            aria-label="Strict targets"
                        />
                    </div>
                </CardContent>
            </Card>

            {!driftLoading && uncataloged && (
                <DriftPanel
                    result={uncataloged}
                    strictTargets={strictTargets}
                    hasDrift={hasDrift}
                />
            )}
        </div>
    );
}

interface DriftPanelProps {
    result: { since: string; total_sends: number; targets: UncatalogedTarget[] };
    strictTargets: boolean;
    hasDrift: boolean;
}

/**
 * "What would turning this on break?" — answered from the sends themselves, so
 * it is never a blind switch. It reads usefully in both states: with the gate
 * off it is a to-do list, with it on a non-empty list would mean sends are
 * being rejected right now.
 */
function DriftPanel({ result, strictTargets, hasDrift }: DriftPanelProps) {
    const days = Math.max(
        1,
        Math.round(
            (Date.now() - new Date(result.since).getTime()) / 86_400_000
        )
    );

    if (!hasDrift) {
        return (
            <Card>
                <CardContent>
                    <h2 className="label-medium mb-1">Uncataloged targets</h2>
                    <p className="text-text-muted paragraph">
                        Every target sent in the last {days} days is in your
                        catalog.{" "}
                        {strictTargets
                            ? "Strict targets is rejecting nothing."
                            : "Turning strict targets on would reject nothing you're currently sending."}
                    </p>
                </CardContent>
            </Card>
        );
    }

    return (
        <Card>
            <CardContent>
                <h2 className="label-medium mb-1">Uncataloged targets</h2>
                <p className="text-text-muted paragraph mb-4">
                    <strong>{result.total_sends}</strong>{" "}
                    {result.total_sends === 1 ? "send" : "sends"} in the last{" "}
                    {days} days named a target with no project preference.{" "}
                    {strictTargets ? (
                        <>
                            Strict targets is on, so sends like these are being
                            rejected — these ran before it was enabled.
                        </>
                    ) : (
                        <>
                            Catalog these before turning strict targets on, or
                            they will start returning <code>400</code>.
                        </>
                    )}
                </p>

                <div className="overflow-x-auto">
                    <table className="w-full text-left">
                        <thead className="text-text-muted label-small">
                            <tr>
                                <th className="py-1 pr-4">Target</th>
                                <th className="py-1 pr-4">Medium</th>
                                <th className="py-1 pr-4">Sends</th>
                                <th className="py-1">Last sent</th>
                            </tr>
                        </thead>
                        <tbody>
                            {result.targets.map((t) => (
                                <tr
                                    key={`${t.channel}/${t.topic}/${t.event}/${t.medium}`}
                                    className="border-border-subtle border-t"
                                >
                                    <td className="py-2 pr-4">
                                        <code>
                                            {t.channel}/{t.topic}/{t.event}
                                        </code>
                                    </td>
                                    <td className="py-2 pr-4">
                                        <Tag variant="muted" size="small">
                                            {PREFERENCE_MEDIUM_LABELS[
                                                t.medium
                                            ] ?? t.medium}
                                        </Tag>
                                    </td>
                                    <td className="py-2 pr-4">{t.sends}</td>
                                    <td className="text-text-muted py-2">
                                        {new Date(
                                            t.last_sent_at
                                        ).toLocaleDateString()}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>

                {/*
                  * A wildcard entry is the answer for runtime-generated topics,
                  * and it is worth saying here: the obvious reading of this
                  * table is "catalog every row", which for a per-resource topic
                  * would mean one entry per post id forever.
                  */}
                <p className="text-text-muted paragraph mt-4">
                    A <code>topic: any</code> entry covers every concrete topic
                    beneath it, so one entry like{" "}
                    <code>post/any/comment</code> catalogs{" "}
                    <code>post/123/comment</code> and every other post — you do
                    not need a row per id.
                </p>
            </CardContent>
        </Card>
    );
}
