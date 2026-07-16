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
    Tooltip,
} from "netra";

import {
    NotificationKind,
    Notification,
    NotificationStatus,
    BroadcastStatus,
    DeliveryStatus,
    BroadcastListItem,
} from "@/features/notification/notification_types";
import { SendNotificationModal } from "@/features/notification/components/send_notification_modal";
import { NotificationKindToggle } from "@/features/notification/components/notification_kind_toggle";
import { EmailDeliveryOverview } from "@/features/notification/components/email_delivery_overview";
import { DeliveryDetailDialog } from "@/features/notification/components/delivery_detail_dialog";
import { deliveryOutcomeText } from "@/features/notification/delivery_copy";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useNotifications,
    useBroadcasts,
} from "@/features/notification/notification_hooks";
import { StatusTag } from "@/components/status_tag";
import { targetToString } from "@/lib/utils";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";

export function NotificationList() {
    useDocumentTitle("Notifications  • Bodhveda");

    const projectID = useGetProjectIDFromParams();
    const [notificationKind, setNotificationKind] =
        useState<NotificationKind>("direct");
    const isViewingNotifications = notificationKind === "direct";
    const isViewingBroadcasts = notificationKind === "broadcast";

    const [directTableState, setDirectTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const {
        data: notificationsData,
        isFetching: isFetchingNotifications,
        isLoading: isLoadingNotifications,
        isError: isErrorNotifications,
    } = useNotifications(
        projectID,
        notificationKind,
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
                        not attempted any email. */}
                    <EmailDeliveryOverview />
                    <NotificationTable
                        key="direct"
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
    }, [
        isViewingNotifications,
        isViewingBroadcasts,
        isErrorNotifications,
        isLoadingNotifications,
        notificationsData?.data?.notifications,
        notificationsData?.data?.pagination.total_items,
        directTableState,
        isFetchingNotifications,
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
                    setKind={setNotificationKind}
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

// NotificationStatusCell renders the in-app outcome and, when the send carried
// email, the email medium's outcome beneath it — so a diverging result (in-app
// delivered, email muted) is visible per row.
//
// An email that did not deliver states WHY inline: `muted` alone is ambiguous
// between "the project never cataloged email for this target" and "the recipient
// opted out", which are opposite fixes. That distinction is why failure_reason
// exists (Phase 4) and it was previously only legible in the post-send toast.
function NotificationStatusCell({
    notification,
}: {
    notification: Notification;
}) {
    const createdAt = new Date(notification.created_at);
    const email = notification.email;

    const inAppLine = (
        <MediumStatusLine
            label="In-app"
            status={notification.status}
            elapsed={
                notification.completed_at
                    ? formatDuration(createdAt, new Date(notification.completed_at))
                    : null
            }
            pending={!notification.completed_at}
        />
    );

    // Only the direct table has email (email is direct-only), and only when the
    // send carried an email block.
    if (!email) return inAppLine;

    // Prefer the webhook-confirmed delivery time; fall back to the
    // provider-accepted (sent) time while delivery is unconfirmed.
    const emailTime = email.delivered_at ?? email.sent_at ?? null;
    const outcome = deliveryOutcomeText(email.status, email.failure_reason);

    return (
        <div className="space-y-1">
            {inAppLine}
            <MediumStatusLine
                label="Email"
                status={email.status}
                elapsed={
                    emailTime
                        ? formatDuration(createdAt, new Date(emailTime))
                        : null
                }
                pending={
                    email.status === "pending" || email.status === "sending"
                }
                reason={outcome}
            />
        </div>
    );
}

function MediumStatusLine({
    label,
    status,
    elapsed,
    pending,
    reason,
}: {
    label: string;
    status: NotificationStatus | BroadcastStatus | DeliveryStatus;
    elapsed: string | null;
    pending?: boolean;
    /** Human copy for a non-delivering outcome; shown inline beside the tag. */
    reason?: { short: string; long: string } | null;
}) {
    return (
        <span className="flex-x gap-x-2">
            <span className="text-xs text-text-muted w-14 shrink-0">
                {label}
            </span>
            <StatusTag status={status} />
            {elapsed ? (
                <span className="text-xs text-text-muted">{elapsed}</span>
            ) : pending ? (
                <Loading size={18} />
            ) : null}

            {/* The short phrase makes the row self-explanatory at a glance; the
                tooltip carries the full explanation and the fix. */}
            {reason && (
                <Tooltip content={reason.long}>
                    <span className="flex-x gap-x-1 text-xs text-text-muted">
                        <IconInfo size={12} />
                        {reason.short}
                    </span>
                </Tooltip>
            )}
        </span>
    );
}

// DeliveryDetailCell is the row-level trigger for the detail dialog. It lives in
// its own column rather than on the Status cell's email line because the dialog
// is NOTIFICATION-scoped (in-app + email), not email-scoped — so every row gets
// one, including the in-app-only sends that are still the common case.
function DeliveryDetailCell({ notification }: { notification: Notification }) {
    const [open, setOpen] = useState(false);

    return (
        <>
            <button
                type="button"
                onClick={() => setOpen(true)}
                className="text-text-muted hover:text-text-primary cursor-pointer text-xs underline underline-offset-2"
            >
                Details
            </button>

            <DeliveryDetailDialog
                notification={notification}
                open={open}
                setOpen={setOpen}
            />
        </>
    );
}

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
