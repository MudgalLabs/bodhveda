import { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import {
    DataTable,
    DataTableColumnHeader,
    DataTableSmart,
    ErrorMessage,
    formatDate,
    formatNumber,
    IconInfo,
    Label,
    LoadingScreen,
    ToggleGroup,
    ToggleGroupItem,
    toast,
    Tooltip,
    WithLabel,
} from "netra";

import {
    useGetProject,
    useGetProjectIDFromParams,
    useUpdateProject,
} from "@/features/project/project_hooks";
import { useGetCatalogDrift } from "@/features/preference/preference_hooks";
import {
    mediumLabel,
    UncatalogedTarget,
} from "@/features/preference/preference_type";
import { targetToString } from "@/lib/utils";

/** What strict targets does, in the one place it can be read at leisure. */
function StrictTargetsHelp() {
    return (
        <div className="space-y-2">
            <p>
                A target is cataloged once it has a project preference. Strict
                targets rejects any send naming a target that doesn't, with a
                400, for the mediums that send asks for.
            </p>
            <p>
                Leave it off while you're still finding your targets. Turn it on
                once your catalog is seeded from your deploy and you'd rather a
                typo fail than deliver.
            </p>
            <p>
                A <code>topic: any</code> preference covers every topic beneath
                it, so <code>post/any/comment</code> is enough for every post —
                a target with a per-resource topic never needs an entry per id.
            </p>
            <p>
                Sends without a target are never rejected, whichever way this is
                set.
            </p>
        </div>
    );
}

export function TargetingSettings() {
    const id = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useGetProject(id);

    if (isError) {
        return <ErrorMessage errorMsg="Error loading targeting settings" />;
    }

    if (isLoading || !data) {
        return <LoadingScreen />;
    }

    return <TargetingForm name={data.data.name} strict={data.data.strict_targets} />;
}

interface TargetingFormProps {
    name: string;
    strict: boolean;
}

function TargetingForm({ name, strict }: TargetingFormProps) {
    const id = useGetProjectIDFromParams();

    const { mutate, isPending } = useUpdateProject({
        onSuccess: () => {
            toast.success(
                strict ? "Strict targets turned off" : "Strict targets turned on"
            );
        },
    });

    // The name rides along because the update payload requires it; a
    // toggle-only PATCH is rejected.
    const onChange = (next: boolean) =>
        mutate({ id: Number(id), name, strict_targets: next });

    return (
        <div>
            <div className="border-border-subtle bg-surface-1 flex max-w-2xl flex-col gap-5 rounded-md border p-5">
                <WithLabel
                    Label={
                        <Label className="flex-x">
                            Strict targets
                            <Tooltip content={<StrictTargetsHelp />}>
                                <IconInfo />
                            </Tooltip>
                        </Label>
                    }
                >
                    <ToggleGroup
                        className="[&_*]:h-8 pl-0!"
                        type="single"
                        size="small"
                        disabled={isPending}
                        value={strict ? "enabled" : "disabled"}
                        onValueChange={(value) =>
                            value && onChange(value === "enabled")
                        }
                    >
                        <ToggleGroupItem value="enabled">
                            Enabled
                        </ToggleGroupItem>

                        <ToggleGroupItem value="disabled">
                            Disabled
                        </ToggleGroupItem>
                    </ToggleGroup>
                </WithLabel>

                {/*
                  * One line, and it states the CONSEQUENCE rather than
                  * restating the control. Off is the state that needs saying
                  * out loud: it reads as "no feature" but it means this project
                  * will send targets nobody can mute.
                  */}
                <p className="label-muted">
                    {strict
                        ? "Sends naming a target with no project preference are rejected."
                        : "Sends naming a target with no project preference are accepted, and no recipient can mute them."}
                </p>
            </div>

            <UncatalogedTargets strict={strict} />
        </div>
    );
}

/**
 * The targets this project has sent but never cataloged.
 *
 * Strict targets is off by default, so nothing forces anyone to notice that
 * their code names targets their catalog doesn't define. This table is what
 * does, and it doubles as the pre-flight for turning the setting on.
 */
function UncatalogedTargets({ strict }: { strict: boolean }) {
    const id = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useGetCatalogDrift(id);

    const days = useMemo(() => {
        if (!data) return 30;
        return Math.max(
            1,
            Math.round(
                (Date.now() - new Date(data.data.since).getTime()) / 86_400_000
            )
        );
    }, [data]);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading uncataloged targets" />;
        }

        if (isLoading || !data) {
            return <LoadingScreen />;
        }

        const result = data.data;

        if (result.total_sends === 0) {
            return (
                <p className="label-muted max-w-2xl">
                    Every target sent in the last {days} days is in your catalog.
                </p>
            );
        }

        return (
            <>
                <p className="label-muted mb-4 max-w-2xl">
                    {formatNumber(result.total_sends)}{" "}
                    {result.total_sends === 1 ? "send" : "sends"} in the last{" "}
                    {days} days named a target with no project preference.{" "}
                    {strict
                        ? "These ran before strict targets was turned on. Sends like them are rejected now."
                        : "Create a preference for each before turning strict targets on."}
                </p>

                <DataTableSmart data={result.targets} columns={columns}>
                    {(table) => <DataTable table={table} />}
                </DataTableSmart>
            </>
        );
    }, [data, days, isError, isLoading, strict]);

    return (
        <>
            <h2 className="paragraph font-medium mt-8 mb-2">
                Uncataloged targets
            </h2>
            {content}
        </>
    );
}

const columns: ColumnDef<UncatalogedTarget>[] = [
    {
        accessorKey: "target",
        header: () => <DataTableColumnHeader title="Target" />,
        cell: ({ row }) => targetToString(row.original.target),
    },
    {
        accessorKey: "medium",
        header: () => <DataTableColumnHeader title="Medium" />,
        cell: ({ row }) => mediumLabel(row.original.medium),
    },
    {
        accessorKey: "sends",
        header: () => <DataTableColumnHeader title="Sends" />,
        cell: ({ row }) => formatNumber(row.original.sends),
    },
    {
        accessorKey: "last_sent_at",
        header: () => <DataTableColumnHeader title="Last sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.last_sent_at), { time: true }),
    },
];
