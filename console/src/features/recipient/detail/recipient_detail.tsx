import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
    buttonVariants,
    DataTable,
    DataTablePagination,
    DataTableSmart,
    DataTableState,
    ErrorMessage,
    formatDate,
    IconArrowLeft,
    IconInfo,
    IconMegaphone,
    IconTarget,
    IconUsers,
    Loading,
    LoadingScreen,
    PageHeading,
    Separator,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    Tag,
    Tooltip,
    useDocumentTitle,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetRecipient } from "@/features/recipient/recipient_hooks";
import { useGetRecipientPreferences } from "@/features/preference/preference_hooks";
import { mediumLabel } from "@/features/preference/preference_type";
import { RecipientContactsPanel } from "@/features/recipient/detail/recipient_contacts_panel";
import { RecipientNotificationsPanel } from "@/features/recipient/detail/recipient_notifications_panel";
import { RecipientListItem } from "@/features/recipient/recipient_types";
import { targetToString } from "@/lib/utils";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";

interface RecipientDetailProps {
    recipientID: string;
}

export function RecipientDetail({ recipientID }: RecipientDetailProps) {
    useDocumentTitle(`${recipientID} • Recipients • Bodhveda`);

    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isError, isFetching } = useGetRecipient(
        projectID,
        recipientID
    );

    const recipient = data?.data;

    const content = useMemo(() => {
        if (isError) {
            return (
                <ErrorMessage errorMsg={`Could not load "${recipientID}"`} />
            );
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!recipient) {
            return <ErrorMessage errorMsg={`Recipient "${recipientID}" not found`} />;
        }

        return (
            <Tabs defaultValue="overview">
                <TabsList>
                    <TabsTrigger value="overview">Overview</TabsTrigger>
                    <TabsTrigger value="notifications">
                        Notifications
                    </TabsTrigger>
                    <TabsTrigger value="preferences">Preferences</TabsTrigger>
                    <TabsTrigger value="contacts">Contacts</TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="pt-4">
                    <OverviewTab recipient={recipient} />
                </TabsContent>

                <TabsContent value="notifications" className="pt-4">
                    <RecipientNotificationsPanel
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>

                <TabsContent value="preferences" className="pt-4">
                    <PreferencesTab
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>

                <TabsContent value="contacts" className="pt-4">
                    <RecipientContactsPanel
                        projectID={projectID}
                        recipientID={recipientID}
                    />
                </TabsContent>
            </Tabs>
        );
    }, [isError, isLoading, recipient, recipientID, projectID]);

    return (
        <div>
            <div className="mb-2">
                <Link
                    to="/projects/$id/recipients"
                    params={{ id: projectID }}
                    className={buttonVariants({
                        variant: "ghost",
                        size: "small",
                    })}
                >
                    <IconArrowLeft size={16} />
                    Recipients
                </Link>
            </div>

            <PageHeading>
                <IconUsers size={18} />
                <h1 className="select-text!">{recipientID}</h1>
                {isFetching && <Loading />}
            </PageHeading>

            {content}
        </div>
    );
}

function OverviewTab({ recipient }: { recipient: RecipientListItem }) {
    return (
        <div className="max-w-2xl space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <Field label="Recipient ID">
                    <span className="select-text!">{recipient.id}</span>
                </Field>

                <Field label="Name">
                    {recipient.name || (
                        <span className="text-foreground-muted">—</span>
                    )}
                </Field>

                <Field label="Created">
                    {formatDate(new Date(recipient.created_at), { time: true })}
                </Field>
            </div>

            <Separator />

            <div>
                <p className="text-foreground-muted text-xs mb-2">
                    Notifications
                </p>
                <div className="flex-x gap-x-6!">
                    <Tooltip content="Direct notifications sent to this recipient">
                        <span className="flex-x">
                            <IconTarget size={16} />
                            {recipient.direct_notifications_count} direct
                        </span>
                    </Tooltip>
                    <Tooltip content="Broadcast notifications this recipient received">
                        <span className="flex-x">
                            <IconMegaphone size={16} />
                            {recipient.broadcast_notifications_count} broadcast
                        </span>
                    </Tooltip>
                </div>
            </div>
        </div>
    );
}

function Field({
    label,
    children,
}: {
    label: string;
    children: React.ReactNode;
}) {
    return (
        <div>
            <p className="text-foreground-muted text-xs mb-1">{label}</p>
            <div className="text-sm">{children}</div>
        </div>
    );
}

// PreferencesTab is READ-ONLY in Phase 9.2 — the editable per-(target, medium)
// grid is 9.3. It shows the RESOLVED state (the project catalog overlaid with
// this recipient's overrides), because a recipient with no stored row is not
// "unset": they are following the project default, which `inherited` says.
function PreferencesTab({
    projectID,
    recipientID,
}: {
    projectID: string;
    recipientID: string;
}) {
    const { data, isLoading, isError } = useGetRecipientPreferences(
        projectID,
        recipientID
    );

    const preferences = data?.data?.preferences ?? [];

    const [tableState, setTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    if (isError) {
        return <ErrorMessage errorMsg="Error loading preferences" />;
    }

    if (isLoading) {
        return <Loading />;
    }

    if (preferences.length === 0) {
        return (
            <p className="text-foreground-muted text-sm">
                This project has no preference catalog yet, so there is nothing
                for this recipient to opt in or out of. Create a project
                preference to define what can be subscribed to.
            </p>
        );
    }

    return (
        <div className="space-y-4">
            <p className="text-foreground-muted text-sm">
                What this recipient currently receives, per target and medium.
                <strong> Inherited</strong> means they have no preference of
                their own and follow the project default. Read-only for now.
            </p>

            <DataTableSmart
                columns={preferenceColumns}
                data={preferences}
                total={preferences.length}
                state={tableState}
                onStateChange={setTableState}
            >
                {(table) => (
                    <div className="space-y-4">
                        <DataTable table={table} />
                        {preferences.length > tableState.pagination.pageSize && (
                            <DataTablePagination
                                table={table}
                                total={preferences.length}
                            />
                        )}
                    </div>
                )}
            </DataTableSmart>
        </div>
    );
}

const preferenceColumns = [
    {
        id: "target",
        header: () => (
            <TargetInfoTooltip>
                <span className="flex-x w-fit">
                    Target <IconInfo />
                </span>
            </TargetInfoTooltip>
        ),
        cell: ({ row }: { row: { original: PreferenceRow } }) =>
            targetToString(row.original.target),
    },
    {
        id: "label",
        header: () => <span>Label</span>,
        cell: ({ row }: { row: { original: PreferenceRow } }) =>
            row.original.target.label ?? (
                <span className="text-foreground-muted">—</span>
            ),
    },
    {
        id: "medium",
        header: () => <span>Medium</span>,
        cell: ({ row }: { row: { original: PreferenceRow } }) =>
            mediumLabel(row.original.target.medium),
    },
    {
        id: "state",
        header: () => <span>Receives</span>,
        cell: ({ row }: { row: { original: PreferenceRow } }) => (
            <div className="flex-x">
                <Tag
                    variant={row.original.state.enabled ? "success" : "default"}
                >
                    {row.original.state.enabled ? "Yes" : "No"}
                </Tag>
                {row.original.state.inherited && (
                    <Tooltip content="No preference of their own — following the project default.">
                        <Tag variant="muted" size="small">
                            Inherited
                        </Tag>
                    </Tooltip>
                )}
            </div>
        ),
    },
];

type PreferenceRow = {
    target: {
        channel: string;
        topic: string;
        event: string;
        medium: string;
        label?: string;
    };
    state: { enabled: boolean; inherited: boolean };
};
