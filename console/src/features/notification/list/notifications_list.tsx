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
    formatDuration,
    LoadingScreen,
    useDocumentTitle,
    IconInfo,
} from "netra";

import {
    NotificationKind,
    Notification,
    BroadcastListItem,
    NotificationFilters,
} from "@/features/notification/notification_types";
import { SendNotificationModal } from "@/features/notification/components/send_notification_modal";
import { NotificationKindToggle } from "@/features/notification/components/notification_kind_toggle";
import { NotificationFilterBar } from "@/features/notification/components/notification_filter_bar";
import { notificationFiltersToParams } from "@/features/notification/notification_filters";
import { EmailDeliveryOverview } from "@/features/notification/components/email_delivery_overview";
import {
    DeliveryDetailCell,
    MediumStatusLine,
    NotificationStatusCell,
} from "@/features/notification/components/notification_cells";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useNotifications,
    useBroadcasts,
} from "@/features/notification/notification_hooks";
import { RecipientLink } from "@/features/recipient/recipient_link";
import { targetToString } from "@/lib/utils";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";

interface NotificationListProps {
    /**
     * The filter selection, `kind` included. Owned by the route, which reads it
     * from the URL — so a filtered view is shareable and survives a reload.
     */
    filters: NotificationFilters;
    onFiltersChange: (filters: NotificationFilters) => void;
}

export function NotificationList({
    filters,
    onFiltersChange,
}: NotificationListProps) {
    useDocumentTitle("Notifications  • Bodhveda");

    const projectID = useGetProjectIDFromParams();
    const notificationKind = filters.kind;
    const isViewingNotifications = notificationKind === "direct";
    const isViewingBroadcasts = notificationKind === "broadcast";

    const [directTableState, setDirectTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    // Narrowing the result set invalidates the page you were on: filtering while
    // on page 5 of an unfiltered list would otherwise land you on page 5 of a
    // shorter one, which renders as an empty table that reads like "no matches".
    const handleFiltersChange = (next: NotificationFilters) => {
        setDirectTableState((s) => ({
            ...s,
            pagination: { ...s.pagination, pageIndex: 0 },
        }));
        onFiltersChange(next);
    };

    // Resetting the page above is only half the job, and the missing half is not
    // obvious: DataTableSmart seeds its pagination from `state.pagination` ONCE
    // (useState initializer) and thereafter owns it, publishing changes upward
    // via onStateChange but never reading the prop again. So the reset reaches
    // the QUERY but not the table's own pager, which goes on claiming "Page 4"
    // over page-1 rows. Remounting on a filter change is what makes the table
    // re-seed from the state we just reset. No other console list hits this,
    // because nothing else ever moved their page from the outside.
    //
    // Keyed on the filters ONLY — never the page, or paging would remount the
    // table and bounce it back to page 1 on every click.
    const directTableKey = `direct:${JSON.stringify(
        notificationFiltersToParams(filters)
    )}`;

    const {
        data: notificationsData,
        isFetching: isFetchingNotifications,
        isLoading: isLoadingNotifications,
        isError: isErrorNotifications,
    } = useNotifications(projectID, {
        kind: notificationKind,
        page: directTableState.pagination.pageIndex + 1,
        limit: directTableState.pagination.pageSize,
        filters,
    });

    const [broadcastTableState, setBroadcastTableState] =
        useState<DataTableState>({
            columnVisibility: {},
            pagination: { pageIndex: 0, pageSize: 10 },
            sorting: [],
        });

    const {
        data: broadcastsData,
        isFetching: isFetchingBroadcasts,
        isLoading: isLoadingBroadcasts,
        isError: isErrorBroadcasts,
    } = useBroadcasts(
        projectID,
        broadcastTableState.pagination.pageIndex + 1,
        broadcastTableState.pagination.pageSize
    );

    const content = useMemo(() => {
        if (isViewingNotifications) {
            if (isErrorNotifications) {
                return <ErrorMessage errorMsg="Error loading notifications" />;
            }

            if (isLoadingNotifications) {
                return <LoadingScreen />;
            }

            return (
                <>
                    {/* Email is DIRECT-only, so its delivery analytics live above
                        the direct table. The card self-hides when the project has
                        not attempted any email. It is deliberately NOT filtered —
                        it is the project's lifetime email picture (Phase 5), and
                        silently re-scoping it to the current filter would make
                        two different numbers look like the same one. */}
                    <EmailDeliveryOverview />
                    <NotificationFilterBar
                        filters={filters}
                        onChange={handleFiltersChange}
                    />
                    <NotificationTable
                        key={directTableKey}
                        data={notificationsData?.data?.notifications || []}
                        totalItems={
                            notificationsData?.data?.pagination.total_items || 0
                        }
                        state={directTableState}
                        onStateChange={setDirectTableState}
                        isFetching={isFetchingNotifications}
                    />
                </>
            );
        } else if (isViewingBroadcasts) {
            if (isErrorBroadcasts && isViewingBroadcasts) {
                return <ErrorMessage errorMsg="Error loading broadcasts" />;
            }

            if (isLoadingBroadcasts) {
                return <LoadingScreen />;
            }

            return (
                <BroadcastsTable
                    key="broadcast"
                    data={broadcastsData?.data?.broadcasts || []}
                    totalItems={
                        broadcastsData?.data?.pagination.total_items || 0
                    }
                    state={broadcastTableState}
                    onStateChange={setBroadcastTableState}
                    isFetching={isFetchingBroadcasts}
                />
            );
        }

        return null;
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [
        isViewingNotifications,
        isViewingBroadcasts,
        isErrorNotifications,
        isLoadingNotifications,
        notificationsData?.data?.notifications,
        notificationsData?.data?.pagination.total_items,
        directTableState,
        isFetchingNotifications,
        filters,
        directTableKey,
        isErrorBroadcasts,
        isLoadingBroadcasts,
        broadcastsData?.data?.broadcasts,
        broadcastsData?.data?.pagination.total_items,
        broadcastTableState,
        isFetchingBroadcasts,
    ]);

    return (
        <div>
            <PageHeading>
                <IconBell size={18} />
                <h1>Notifications</h1>
                {(isFetchingNotifications || isFetchingBroadcasts) && (
                    <Loading />
                )}
            </PageHeading>

            <div className="flex justify-between mb-4">
                <NotificationKindToggle
                    kind={notificationKind}
                    // Switching kind PRESERVES the filters rather than clearing
                    // them: the broadcast table is served by a different endpoint
                    // and shows no filter bar, so the selection is simply
                    // dormant, and switching back restores it intact.
                    setKind={(kind: NotificationKind) =>
                        handleFiltersChange({ ...filters, kind })
                    }
                />

                <SendNotificationModal
                    renderTrigger={() => (
                        <Button>
                            <IconSend size={16} />
                            Send Notification
                        </Button>
                    )}
                />
            </div>

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
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
        cell: ({ row }) => (
            <RecipientLink recipientID={row.original.recipient_id} />
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
        accessorKey: "state.opened",
        header: () => <DataTableColumnHeader title="Opened" />,
        cell: ({ row }) => (row.original.state.opened ? "Yes" : "No"),
    },
    {
        accessorKey: "status",
        header: () => <DataTableColumnHeader title="Status" />,
        cell: ({ row }) => <NotificationStatusCell notification={row.original} />,
    },
    {
        id: "details",
        header: () => <DataTableColumnHeader title="" />,
        cell: ({ row }) => <DeliveryDetailCell notification={row.original} />,
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
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Sent" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
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
        accessorKey: "status",
        header: () => <DataTableColumnHeader title="Status" />,
        cell: ({ row }) => {
            const completedAt = row.original.completed_at
                ? new Date(row.original.completed_at)
                : null;
            const createdAt = new Date(row.original.created_at);

            return (
                <MediumStatusLine
                    label="In-app"
                    status={row.original.status}
                    elapsed={
                        completedAt
                            ? formatDuration(createdAt, completedAt)
                            : null
                    }
                    pending={!completedAt}
                />
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
