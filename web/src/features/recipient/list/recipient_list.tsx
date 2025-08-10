import { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
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
    IconEdit,
    IconEllipsis,
    IconInfo,
    IconPlus,
    IconTrash,
    PageHeading,
    toast,
    Tooltip,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import {
    useDeleteRecipient,
    useGetRecipients,
} from "@/features/recipient/recipient_hooks";
import { CreateRecipientModal } from "@/features/recipient/list/create_recipient_modal";
import { RecipientListItem } from "@/features/recipient/recipient_types";
import { EditRecipientModal } from "./edit_recipient_modal";

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

function ActionCell({ recipient }: { recipient: RecipientListItem }) {
    const [dropdownOpen, setDropdownOpen] = useState(false);
    const [editOpen, setEditOpen] = useState(false);

    const handleEditOpen = () => {
        setDropdownOpen(false);
        setEditOpen(true);
    };

    const projectID = useGetProjectIDFromParams();

    const { mutate: deleteRecipient, isPending: isDeleting } =
        useDeleteRecipient(projectID, {
            onSuccess: () => {
                toast.success("Recipient deleted successfully");
            },
        });

    return (
        <>
            <DropdownMenu open={dropdownOpen} onOpenChange={setDropdownOpen}>
                <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon">
                        <IconEllipsis />
                    </Button>
                </DropdownMenuTrigger>

                <DropdownMenuContent>
                    <DropdownMenuItem asChild>
                        <Button
                            variant="ghost"
                            className="w-full!"
                            onClick={handleEditOpen}
                        >
                            <IconEdit size={16} />
                            Edit
                        </Button>
                    </DropdownMenuItem>

                    <DropdownMenuItem asChild>
                        <Button
                            variant="destructive"
                            className="w-full!"
                            onClick={() =>
                                deleteRecipient({
                                    recipientID: recipient.recipient_id,
                                })
                            }
                            disabled={isDeleting}
                        >
                            <IconTrash size={16} />
                            Delete
                        </Button>
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>

            <EditRecipientModal
                recipient={recipient}
                open={editOpen}
                setOpen={setEditOpen}
            />
        </>
    );
}

const columns: ColumnDef<RecipientListItem>[] = [
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
        cell: ({ row }) => (
            <span className="select-text!">{row.original.recipient_id}</span>
        ),
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
        cell: ({ row }) => <ActionCell recipient={row.original} />,
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
