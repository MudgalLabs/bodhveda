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
} from "netra";

import {
    NotificationKind,
    Notification,
} from "@/features/notification/notification_types";
import { SendNotificationModal } from "@/features/notification/components/send_notification_modal";
import { NotificationKindToggle } from "@/features/notification/components/notification_kind_toggle";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetNotifications } from "@/features/notification/notification_hooks";

export function NotificationList() {
    const projectID = useGetProjectIDFromParams();
    const [kind, setKind] = useState<NotificationKind>("direct");
    const isDirect = kind === "direct";

    const [directTableState, setDirectTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const [broadcastTableState, setBroadcastTableState] =
        useState<DataTableState>({
            columnVisibility: {},
            pagination: { pageIndex: 0, pageSize: 10 },
            sorting: [],
        });

    const { data, isFetching, isError } = useGetNotifications(
        projectID,
        kind,
        isDirect
            ? directTableState.pagination.pageIndex + 1
            : broadcastTableState.pagination.pageIndex + 1,
        isDirect
            ? directTableState.pagination.pageSize
            : broadcastTableState.pagination.pageSize
    );

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading notifications" />;
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
                    <Table
                        key="direct"
                        data={data?.data?.notifications || []}
                        totalItems={data?.data?.pagination.total_items || 0}
                        state={directTableState}
                        onStateChange={setDirectTableState}
                        isFetching={isFetching}
                    />
                ) : (
                    <Table
                        key="broadcast"
                        data={data?.data?.notifications || []}
                        totalItems={data?.data?.pagination.total_items || 0}
                        state={broadcastTableState}
                        onStateChange={setBroadcastTableState}
                        isFetching={isFetching}
                    />
                )}
            </>
        );
    }, [
        isError,
        kind,
        data?.data?.notifications,
        data?.data?.pagination.total_items,
        isDirect,
        directTableState,
        broadcastTableState,
        isFetching,
    ]);

    return (
        <div>
            <PageHeading>
                <IconBell size={18} />
                <h1>Notifications</h1>
                {isFetching && <Loading />}
            </PageHeading>

            {content}
        </div>
    );
}

const columns: ColumnDef<Notification>[] = [
    {
        accessorKey: "id",
        header: () => <DataTableColumnHeader title="ID" />,
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
        header: () => <DataTableColumnHeader title="Delivered" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
];

interface TableProps {
    data: Notification[];
    totalItems: number;
    state: DataTableState;
    onStateChange?: (state: DataTableState) => void;
    isFetching?: boolean;
}

function Table(props: TableProps) {
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
