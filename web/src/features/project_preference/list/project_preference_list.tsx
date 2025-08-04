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
import { useGetProjectPreferences } from "@/features/project_preference/project_preference_hooks";
import { CreateProjectPreferenceModal } from "@/features/project_preference/list/create_project_preference_modal";
import { ProjectPreference } from "@/features/project_preference/project_preference_type";

export function ProjectPreferenceList() {
    const id = useGetProjectIDFromParams();
    const { data, isLoading, isError } = useGetProjectPreferences(id);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading preferences" />;
        }

        if (!data) return null;

        return <ListTable data={data.data} />;
    }, [data, isError]);

    return (
        <div>
            <PageHeading heading="Preferences" loading={isLoading} />

            <div className="flex justify-end mb-4">
                <CreateProjectPreferenceModal
                    renderTrigger={() => (
                        <Button>
                            <IconPlus size={16} />
                            Create Preference
                        </Button>
                    )}
                />
            </div>
            {content}
        </div>
    );
}

const columns: ColumnDef<ProjectPreference>[] = [
    {
        accessorKey: "label",
        header: () => <DataTableColumnHeader title="Label" />,
    },
    {
        accessorKey: "default_enabled",
        header: () => <DataTableColumnHeader title="Default" />,
        cell: ({ row }) =>
            row.original.default_enabled ? "Enabled" : "Disabled",
    },
    {
        accessorKey: "channel",
        header: () => <DataTableColumnHeader title="Channel" />,
    },
    {
        accessorKey: "topic",
        header: () => <DataTableColumnHeader title="Topic" />,
    },
    {
        accessorKey: "event",
        header: () => <DataTableColumnHeader title="Event" />,
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
    data: ProjectPreference[];
}

function ListTable({ data }: ListTableProps) {
    return (
        <DataTableSmart data={data} columns={columns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
