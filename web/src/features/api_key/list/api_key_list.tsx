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
    IconPlus,
    IconTrash,
    PageHeading,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetAPIKeys } from "@/features/api_key/api_key_hooks";
import { CreateAPIKeyModal } from "@/features/api_key/list/create_api_key_modal";
import { APIKey, apiKeyScopeToString } from "@/features/api_key/api_key_types";

export function APIKeyList() {
    const id = useGetProjectIDFromParams();

    const { data, isLoading, isError } = useGetAPIKeys(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading projects" />;
        }

        if (!data) return null;

        return <ListTable data={data.data} />;
    }, [data, isError]);

    return (
        <div>
            <PageHeading heading="API Keys" loading={isLoading} />

            <div className="flex justify-end">
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

const columns: ColumnDef<APIKey>[] = [
    {
        accessorKey: "name",
        header: () => <DataTableColumnHeader title="Name" />,
    },
    {
        accessorKey: "token_partial",
        header: () => <DataTableColumnHeader title="Token" />,
        cell: ({ row }) => <pre>{row.original.token_partial}</pre>,
    },
    {
        accessorKey: "scope",
        header: () => <DataTableColumnHeader title="Permission" />,
        cell: ({ row }) => apiKeyScopeToString(row.original.scope),
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
    data: APIKey[];
}

function ListTable({ data }: ListTableProps) {
    return (
        <DataTableSmart data={data} columns={columns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
