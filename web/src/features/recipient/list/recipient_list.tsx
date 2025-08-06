import { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import {
    Button,
    DataTable,
    DataTableColumnHeader,
    DataTableSmart,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
    ErrorMessage,
    formatTimeAgo,
    IconEllipsis,
    IconInfo,
    IconPlus,
    IconTrash,
    PageHeading,
    Tooltip,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetRecipients } from "@/features/recipient/recipient_hooks";
import { CreateRecipientModal } from "@/features/recipient/list/create_recipient_modal";
import { RecipientListItem } from "@/features/recipient/recipient_types";

export function RecipientList() {
    const id = useGetProjectIDFromParams();
    const { data, isLoading, isError } = useGetRecipients(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading recipients" />;
        }

        if (!data) return null;

        return <ListTable data={data.data} />;
    }, [data, isError]);

    return (
        <div>
            <PageHeading heading="Recipients" loading={isLoading} />

            <div className="flex justify-end mb-4">
                <CreateRecipientModal
                    renderTrigger={() => (
                        <Button>
                            <IconPlus size={16} />
                            Create Recipient
                        </Button>
                    )}
                />
            </div>

            {content}
        </div>
    );
}

const columns: ColumnDef<RecipientListItem>[] = [
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
    },
    {
        accessorKey: "name",
        header: () => <DataTableColumnHeader title="Name" />,
    },
    {
        id: "notifications_count",
        header: () => (
            <DataTableColumnHeader
                title={
                    <p className="flex-x">
                        Notifications
                        <Tooltip
                            content={
                                <div className="space-y-2">
                                    <p>
                                        🎯 <strong>Direct notifications</strong>{" "}
                                        are sent to a specific recipient.
                                    </p>
                                    <p>
                                        📢{" "}
                                        <strong>Broadcast notifications</strong>{" "}
                                        are sent to one or more recipients.
                                    </p>
                                </div>
                            }
                        >
                            <IconInfo />
                        </Tooltip>
                    </p>
                }
            />
        ),
        cell: ({ row }) => (
            <div className="flex-x gap-x-6!">
                <p>🎯 {row.original.direct_notifications_count}</p>
                <p>📢 {row.original.broadcast_notifications_count}</p>
            </div>
        ),
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created At" />,
        cell: ({ row }) => formatTimeAgo(new Date(row.original.created_at)),
    },
    {
        id: "actions",
        cell: () => (
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon">
                        <IconEllipsis />
                    </Button>
                </DropdownMenuTrigger>

                <DropdownMenuContent>
                    <DropdownMenuItem>
                        <IconTrash size={16} />
                        Delete
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
        ),
    },
];

interface ListTableProps {
    data: RecipientListItem[];
}

function ListTable({ data }: ListTableProps) {
    return (
        <DataTableSmart data={data} columns={columns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
