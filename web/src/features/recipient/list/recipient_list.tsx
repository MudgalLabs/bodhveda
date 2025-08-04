import { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import {
    Button,
    DataTable,
    DataTableColumnHeader,
    DataTableSmart,
    ErrorMessage,
    formatTimeAgo,
    IconPlus,
    PageHeading,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetRecipients } from "@/features/recipient/recipient_hooks";
import { CreateRecipientModal } from "@/features/recipient/list/create_recipient_modal";
import { Recipient } from "@/features/recipient/recipient_types";

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
            <div className="flex justify-end">
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

const columns: ColumnDef<Recipient>[] = [
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
    },
    {
        accessorKey: "name",
        header: () => <DataTableColumnHeader title="Name" />,
    },
    {
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created At" />,
        cell: ({ row }) => formatTimeAgo(new Date(row.original.created_at)),
    },
];

interface ListTableProps {
    data: Recipient[];
}

function ListTable({ data }: ListTableProps) {
    return (
        <DataTableSmart data={data} columns={columns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
