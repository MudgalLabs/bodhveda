import { useState, useMemo } from "react";
import { ColumnDef } from "@tanstack/react-table";
import {
    Button,
    IconSend,
    PageHeading,
    DataTableColumnHeader,
    formatTimeAgo,
    DataTableState,
    ErrorMessage,
    DataTableSmart,
    DataTable,
    DataTablePagination,
} from "netra";

import { useSidebar } from "@/components/sidebar/sidebar";
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
    const { isOpen, toggleSidebar } = useSidebar();
    const [kind, setKind] = useState<NotificationKind>("direct");

    const [tableState, setTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const { data, isFetching, isError } = useGetNotifications(
        projectID,
        kind,
        tableState.pagination.pageIndex + 1,
        tableState.pagination.pageSize
    );

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading notifications" />;
        }

        if (!data) return null;

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

                <Table
                    data={data.data.notifications}
                    totalItems={data.data.pagination.total_items}
                    state={tableState}
                    onStateChange={setTableState}
                    isFetching={isFetching}
                />
            </>
        );
    }, [data, isError, isFetching, kind, tableState]);

    return (
        <div>
            <PageHeading
                heading="Notifications"
                loading={isFetching}
                isOpen={isOpen}
                toggleSidebar={toggleSidebar}
            />

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
        accessorKey: "state.seen",
        header: () => <DataTableColumnHeader title="Seen" />,
        cell: ({ row }) => (row.original.state.seen ? "Yes" : "No"),
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
        header: () => <DataTableColumnHeader title="When" />,
        cell: ({ row }) => formatTimeAgo(new Date(row.original.created_at)),
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
                    <DataTablePagination table={table} total={totalItems} />
                </div>
            )}
        </DataTableSmart>
    );
}
