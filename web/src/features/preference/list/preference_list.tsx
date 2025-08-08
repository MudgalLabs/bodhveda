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
    formatNumber,
    formatTimeAgo,
    IconEllipsis,
    IconPlus,
    IconTrash,
    PageHeading,
    ToggleGroup,
    ToggleGroupItem,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetPreferences } from "@/features/preference/preference_hooks";
import { CreateProjectPreferenceModal } from "@/features/preference/list/create_project_preference_modal";
import {
    PreferenceKind,
    ProjectPreference,
    RecipientPreference,
} from "@/features/preference/preference_type";

export function ProjectPreferenceList() {
    const id = useGetProjectIDFromParams();
    const [kind, setKind] = useState<PreferenceKind>("project");
    const isProject = kind === "project";

    const { data, isLoading, isError } = useGetPreferences(id, kind);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading preferences" />;
        }

        if (!data) return null;

        if (isProject) {
            return (
                <ProjectPreferenceListTable
                    data={data.data as ProjectPreference[]}
                />
            );
        } else if (!isProject) {
            return (
                <RecipientPreferenceListTable
                    data={data.data as RecipientPreference[]}
                />
            );
        }

        return null;
    }, [data, isError, isProject]);

    return (
        <div>
            <PageHeading heading="Preferences" loading={isLoading} />

            <div className="flex justify-between mb-4">
                <ToggleGroup
                    className="[&_*]:h-8 pl-0!"
                    type="single"
                    size="small"
                    value={kind}
                    onValueChange={(value) =>
                        value && setKind(value as PreferenceKind)
                    }
                >
                    <ToggleGroupItem value="project">Project</ToggleGroupItem>
                    <ToggleGroupItem value="recipient">
                        Recipient
                    </ToggleGroupItem>
                </ToggleGroup>

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

const projectPreferenceColumns: ColumnDef<ProjectPreference>[] = [
    {
        accessorKey: "label",
        header: () => <DataTableColumnHeader title="Label" />,
    },
    {
        accessorKey: "subscribers",
        header: () => <DataTableColumnHeader title="Subscribers" />,
        cell: ({ row }) => formatNumber(row.original.subscribers),
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

interface ProjectPreferenceListTableProps {
    data: ProjectPreference[];
}

function ProjectPreferenceListTable({ data }: ProjectPreferenceListTableProps) {
    return (
        <DataTableSmart data={data} columns={projectPreferenceColumns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}

const recipientPreferenceColumns: ColumnDef<RecipientPreference>[] = [
    {
        accessorKey: "recipient_id",
        header: () => <DataTableColumnHeader title="Recipient ID" />,
    },
    {
        accessorKey: "enabled",
        header: () => <DataTableColumnHeader title="Enabled / Disabled" />,
        cell: ({ row }) => (row.original.enabled ? "Enabled" : "Disabled"),
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
        accessorKey: "updated_at",
        header: () => <DataTableColumnHeader title="Updated At" />,
        cell: ({ row }) => formatTimeAgo(new Date(row.original.updated_at)),
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

interface RecipientPreferenceListTableProps {
    data: RecipientPreference[];
}

function RecipientPreferenceListTable({
    data,
}: RecipientPreferenceListTableProps) {
    return (
        <DataTableSmart data={data} columns={recipientPreferenceColumns}>
            {(table) => <DataTable table={table} />}
        </DataTableSmart>
    );
}
