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
    formatDate,
    IconEllipsis,
    IconKey,
    IconPlus,
    IconTrash,
    Loading,
    LoadingScreen,
    PageHeading,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetAPIKeys } from "@/features/api_key/api_key_hooks";
import { CreateAPIKeyModal } from "@/features/api_key/components/create_api_key_modal";
import { APIKey, apiKeyScopeToString } from "@/features/api_key/api_key_types";
import { DeleteAPIKeyModal } from "../components/delete_api_key_modal";

export function APIKeyList() {
    const id = useGetProjectIDFromParams();

    const { data, isLoading, isFetching, isError } = useGetAPIKeys(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading API keys" />;
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!data) return null;

        return <ListTable data={data.data} />;
    }, [data, isError, isLoading]);

    return (
        <div>
            <PageHeading>
                <IconKey size={18} />
                <h1>API Keys</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <div className="flex justify-end mb-4">
                <CreateAPIKeyModal
                    renderTrigger={() => (
                        <Button>
                            <IconPlus size={16} />
                            Create API Key
                        </Button>
                    )}
                />
            </div>

            {content}
        </div>
    );
}

function ActionCell({ apiKey }: { apiKey: APIKey }) {
    const projectID = useGetProjectIDFromParams();

    const [dropdownOpen, setDropdownOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);

    const handleOpenDeleteConfirm = () => {
        setDropdownOpen(false);
        setDeleteOpen(true);
    };

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
                            variant="destructive"
                            onClick={handleOpenDeleteConfirm}
                        >
                            <IconTrash size={16} />
                            Delete
                        </Button>
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>

            {deleteOpen && (
                <DeleteAPIKeyModal
                    open={deleteOpen}
                    setOpen={setDeleteOpen}
                    projectID={projectID}
                    apiKey={apiKey}
                />
            )}
        </>
    );
}

const columns: ColumnDef<APIKey>[] = [
    {
        accessorKey: "name",
        header: () => <DataTableColumnHeader title="Name" />,
    },
    {
        accessorKey: "token_partial",
        header: () => <DataTableColumnHeader title="Token" />,
        cell: ({ row }) => (
            <pre className="select-text!">{row.original.token_partial}</pre>
        ),
    },
    {
        accessorKey: "scope",
        header: () => <DataTableColumnHeader title="Scope" />,
        cell: ({ row }) => apiKeyScopeToString(row.original.scope),
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        id: "actions",
        cell: ({ row }) => <ActionCell apiKey={row.original} />,
    },
];

interface ListTableProps {
    data: APIKey[];
}

function ListTable({ data }: ListTableProps) {
    return (
        <DataTableSmart data={data} columns={columns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
