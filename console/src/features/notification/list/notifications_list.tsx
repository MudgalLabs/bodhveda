import { useState, useMemo } from "react";
import { ColumnDef } from "@tanstack/react-table";
import {
    Button,
    IconSend,
    PageHeading,
    DataTableColumnHeader,
    DataTableState,
    ErrorMessage,
    DataTableSmart,
    DataTable,
    DataTablePagination,
    IconBell,
    Loading,
    formatDate,
    Tag,
    formatNumber,
} from "netra";

import {
    NotificationKind,
    Notification,
    BroadcastListItem,
} from "@/features/notification/notification_types";
import { SendNotificationModal } from "@/features/notification/components/send_notification_modal";
import { NotificationKindToggle } from "@/features/notification/components/notification_kind_toggle";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useNotifications,
    useBroadcasts,
} from "@/features/notification/notification_hooks";

export function NotificationList() {
    const projectID = useGetProjectIDFromParams();
    const [kind, setKind] = useState<NotificationKind>("direct");
    const isDirect = kind === "direct";
    const isBroadcast = kind === "broadcast";

    const [directTableState, setDirectTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const { data, isFetching, isError } = useNotifications(
        projectID,
        kind,
        directTableState.pagination.pageIndex + 1,
        directTableState.pagination.pageSize
    );

    const [broadcastTableState, setBroadcastTableState] =
        useState<DataTableState>({
            columnVisibility: {},
            pagination: { pageIndex: 0, pageSize: 10 },
            sorting: [],
        });

    const {
        data: broadcastsData,
        isFetching: isBroadcastsFetching,
        isError: isBroadcastsError,
    } = useBroadcasts(
        projectID,
        broadcastTableState.pagination.pageIndex + 1,
        broadcastTableState.pagination.pageSize
    );

    const content = useMemo(() => {
        if (isError && isDirect) {
            return <ErrorMessage errorMsg="Error loading notifications" />;
        }

        if (isBroadcastsError && isBroadcast) {
            return <ErrorMessage errorMsg="Error loading broadcasts" />;
        }

        return (
            <>
                <div className="flex justify-between mb-4">
                    <NotificationKindToggle kind={kind} setKind={setKind} />

                    <SendNotificationModal
                        renderTrigger={() => (
                            <Button>
                                <IconSend size={16} />
                                Send Notification
                            </Button>
                        )}
                    />
                </div>

                {isDirect ? (
                    <NotificationTable
                        key="direct"
                        data={data?.data?.notifications || []}
                        totalItems={data?.data?.pagination.total_items || 0}
                        state={directTableState}
                        onStateChange={setDirectTableState}
                        isFetching={isFetching}
                    />
                ) : (
                    <BroadcastsTable
                        key="broadcast"
                        data={broadcastsData?.data?.broadcasts || []}
                        totalItems={
                            broadcastsData?.data?.pagination.total_items || 0
                        }
                        state={broadcastTableState}
                        onStateChange={setBroadcastTableState}
                        isFetching={isBroadcastsFetching}
                    />
                )}
            </>
        );
    }, [
        isError,
        isDirect,
        isBroadcastsError,
        isBroadcast,
        kind,
        data?.data?.notifications,
        data?.data?.pagination.total_items,
        directTableState,
        isFetching,
        broadcastsData?.data?.broadcasts,
        broadcastsData?.data?.pagination.total_items,
        broadcastTableState,
        isBroadcastsFetching,
    ]);

    return (
        <div>
            <PageHeading>
                <IconBell size={18} />
                <h1>Notifications</h1>
                {(isFetching || isBroadcastsFetching) && <Loading />}
            </PageHeading>

            {content}
        </div>
    );
}

const columns: ColumnDef<Notification>[] = [
    {
        id: "#",
        header: () => <DataTableColumnHeader title="#" />,
        cell: ({ row }) => row.index + 1,
    },
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
    },
    {
        accessorKey: "channel",
        header: () => <DataTableColumnHeader title="Channel" />,
        cell: ({ row }) => row.original.target.channel,
    },
    {
        accessorKey: "topic",
        header: () => <DataTableColumnHeader title="Topic" />,
        cell: ({ row }) => row.original.target.topic,
    },
    {
        accessorKey: "event",
        header: () => <DataTableColumnHeader title="Event" />,
        cell: ({ row }) => row.original.target.event,
    },
    {
        accessorKey: "state.read",
        header: () => <DataTableColumnHeader title="Read" />,
        cell: ({ row }) => (row.original.state.read ? "Yes" : "No"),
    },
    {
        accessorKey: "state.opened",
        header: () => <DataTableColumnHeader title="Opened" />,
        cell: ({ row }) => (row.original.state.opened ? "Yes" : "No"),
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
];

interface NotificationTableProps {
    data: Notification[];
    totalItems: number;
    state: DataTableState;
    onStateChange?: (state: DataTableState) => void;
    isFetching?: boolean;
}

function NotificationTable(props: NotificationTableProps) {
    const { data, totalItems, state, onStateChange, isFetching } = props;
    return (
        <DataTableSmart
            columns={columns}
            data={data}
            total={totalItems}
            state={state}
            onStateChange={onStateChange}
            isFetching={isFetching}
        >
            {(table) => (
                <div className="space-y-4">
                    <DataTable table={table} />
                    {totalItems > state.pagination.pageSize && (
                        <DataTablePagination table={table} total={totalItems} />
                    )}
                </div>
            )}
        </DataTableSmart>
    );
}

const broadcastColumns: ColumnDef<BroadcastListItem>[] = [
    {
        id: "#",
        header: () => <DataTableColumnHeader title="#" />,
        cell: ({ row }) => row.index + 1,
    },
    {
        accessorKey: "target.channel",
        header: () => <DataTableColumnHeader title="Channel" />,
        cell: ({ row }) => row.original.target.channel,
    },
    {
        accessorKey: "target.topic",
        header: () => <DataTableColumnHeader title="Topic" />,
        cell: ({ row }) => row.original.target.topic,
    },
    {
        accessorKey: "target.event",
        header: () => <DataTableColumnHeader title="Event" />,
        cell: ({ row }) => row.original.target.event,
    },
    {
        accessorKey: "delivered_count",
        header: () => <DataTableColumnHeader title="Delivered" />,
    },
    {
        accessorKey: "read_count",
        header: () => <DataTableColumnHeader title="Read" />,
    },
    {
        accessorKey: "opened_count",
        header: () => <DataTableColumnHeader title="Opened" />,
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        id: "status",
        header: () => <DataTableColumnHeader title="Status" />,
        cell: ({ row }) => {
            const completedAt = row.original.completed_at
                ? new Date(row.original.completed_at)
                : null;
            const createdAt = new Date(row.original.created_at);
            let msDiff = 0;

            if (completedAt) {
                msDiff = completedAt.getTime() - createdAt.getTime();
            }

            return completedAt ? (
                <span className="flex-x">
                    <Tag variant="success">Delivered</Tag>
                    <span className="text-xs text-text-muted">
                        {formatNumber(msDiff / 1000)}s
                    </span>
                </span>
            ) : (
                <span className="flex-x gap-x-4">
                    <Tag>Delivering</Tag>
                    <Loading size={18} />
                </span>
            );
        },
    },
];

interface BroadcastsTableProps {
    data: any[];
    totalItems: number;
    state: DataTableState;
    onStateChange?: (state: DataTableState) => void;
    isFetching?: boolean;
}

function BroadcastsTable(props: BroadcastsTableProps) {
    const { data, totalItems, state, onStateChange, isFetching } = props;
    return (
        <DataTableSmart
            columns={broadcastColumns}
            data={data}
            total={totalItems}
            state={state}
            onStateChange={onStateChange}
            isFetching={isFetching}
        >
            {(table) => (
                <div className="space-y-4">
                    <DataTable table={table} />
                    {totalItems > state.pagination.pageSize && (
                        <DataTablePagination table={table} total={totalItems} />
                    )}
                </div>
            )}
        </DataTableSmart>
    );
}
