import { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import {
    Button,
    DataTable,
    DataTableColumnHeader,
    DataTablePagination,
    DataTableSmart,
    DataTableState,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
    ErrorMessage,
    formatDate,
    IconEdit,
    IconEllipsis,
    IconHouse,
    IconInfo,
    IconMegaphone,
    IconPlus,
    IconTarget,
    IconTrash,
    Loading,
    LoadingScreen,
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

    const [tableState, setTableState] = useState<DataTableState>({
        columnVisibility: {},
        pagination: { pageIndex: 0, pageSize: 10 },
        sorting: [],
    });

    const { data, isFetching, isLoading, isError } = useGetRecipients(
        id,
        tableState.pagination.pageIndex + 1,
        tableState.pagination.pageSize
    );

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading recipients" />;
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        return (
            <Table
                data={data?.data?.recipients || []}
                totalItems={data?.data?.pagination.total_items || 0}
                state={tableState}
                onStateChange={setTableState}
                isFetching={isFetching}
            />
        );
    }, [
        data?.data?.pagination.total_items,
        data?.data?.recipients,
        isError,
        isFetching,
        isLoading,
        tableState,
    ]);

    return (
        <div>
            <PageHeading>
                <IconHouse size={18} />
                <h1>Recipients</h1>
                {isFetching && <Loading />}
            </PageHeading>

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
                                    recipientID: recipient.id,
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
        accessorKey: "id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
        cell: ({ row }) => (
            <span className="select-text!">{row.original.id}</span>
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
                                    <span className="flex-x">
                                        <IconTarget size={16} />
                                        Direct notifications
                                    </span>
                                    <span className="flex-x">
                                        <IconMegaphone size={16} />
                                        Broadcast notifications
                                    </span>
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
            <div className="flex-x gap-x-4!">
                <span className="flex-x">
                    <IconTarget size={16} />
                    {row.original.direct_notifications_count}
                </span>
                <span className="flex-x">
                    <IconMegaphone size={16} />
                    {row.original.broadcast_notifications_count}
                </span>
            </div>
        ),
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        id: "actions",
        cell: ({ row }) => <ActionCell recipient={row.original} />,
    },
];

interface TableProps {
    data: RecipientListItem[];
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
