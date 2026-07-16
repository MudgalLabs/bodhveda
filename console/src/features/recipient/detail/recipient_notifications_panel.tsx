import { useState } from "react";
import { ColumnDef } from "@tanstack/react-table";
import {
    DataTable,
    DataTableColumnHeader,
    DataTablePagination,
    DataTableSmart,
    DataTableState,
    ErrorMessage,
    formatDate,
    IconInfo,
    IconMegaphone,
    IconTarget,
    Loading,
    Tooltip,
} from "netra";

import { Notification } from "@/features/notification/notification_types";
import {
    DeliveryDetailCell,
    NotificationStatusCell,
} from "@/features/notification/components/notification_cells";
import { useNotifications } from "@/features/notification/notification_hooks";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";
import { targetToString } from "@/lib/utils";

interface RecipientNotificationsPanelProps {
    projectID: string;
    recipientID: string;
}

/**
 * Everything this project has sent this recipient — direct and broadcast.
 *
 * This is the OPERATOR's view, not the recipient's inbox. The Developer API's
 * recipient feed (`ListForRecipient`) deliberately hides `muted` and
 * `quota_exceeded` because the recipient never received them — but those are
 * exactly the rows someone asking "why didn't they get it?" is looking for, so
 * this reads the console notifications list scoped to one recipient instead.
 */
export function RecipientNotificationsPanel({
    projectID,
    recipientID,
}: RecipientNotificationsPanelProps) {
    const [tableState, setTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const { data, isFetching, isLoading, isError } = useNotifications(
        projectID,
        "all",
        tableState.pagination.pageIndex + 1,
        tableState.pagination.pageSize,
        recipientID
    );

    if (isError) {
        return <ErrorMessage errorMsg="Error loading notifications" />;
    }

    if (isLoading) {
        return <Loading />;
    }

    const notifications = data?.data?.notifications ?? [];
    const totalItems = data?.data?.pagination.total_items ?? 0;

    if (totalItems === 0) {
        return (
            <p className="text-foreground-muted text-sm">
                Nothing has been sent to this recipient yet.
            </p>
        );
    }

    return (
        <DataTableSmart
            columns={columns}
            data={notifications}
            total={totalItems}
            state={tableState}
            onStateChange={setTableState}
            isFetching={isFetching}
        >
            {(table) => (
                <div className="space-y-4">
                    <DataTable table={table} />
                    {totalItems > tableState.pagination.pageSize && (
                        <DataTablePagination table={table} total={totalItems} />
                    )}
                </div>
            )}
        </DataTableSmart>
    );
}

// Mirrors the project Notifications table, minus the Recipient column (every row
// is this recipient) and plus Kind (this feed merges direct + broadcast, which
// the project list keeps on separate tabs).
const columns: ColumnDef<Notification>[] = [
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        id: "kind",
        header: () => <DataTableColumnHeader title="Kind" />,
        cell: ({ row }) =>
            row.original.broadcast_id === null ? (
                <Tooltip content="Direct notification">
                    <span className="flex-x">
                        <IconTarget size={16} />
                        Direct
                    </span>
                </Tooltip>
            ) : (
                <Tooltip content="Received via a broadcast">
                    <span className="flex-x">
                        <IconMegaphone size={16} />
                        Broadcast
                    </span>
                </Tooltip>
            ),
    },
    {
        accessorKey: "target",
        header: () => (
            <DataTableColumnHeader
                title={
                    <TargetInfoTooltip>
                        <span className="flex-x w-fit">
                            Target <IconInfo />
                        </span>
                    </TargetInfoTooltip>
                }
            />
        ),
        cell: ({ row }) => targetToString(row.original.target),
    },
    {
        accessorKey: "state.read",
        header: () => <DataTableColumnHeader title="Read" />,
        cell: ({ row }) => (row.original.state.read ? "Yes" : "No"),
    },
    {
        accessorKey: "status",
        header: () => <DataTableColumnHeader title="Status" />,
        cell: ({ row }) => (
            <NotificationStatusCell notification={row.original} />
        ),
    },
    {
        id: "details",
        header: () => <DataTableColumnHeader title="" />,
        cell: ({ row }) => <DeliveryDetailCell notification={row.original} />,
    },
];
