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
    formatNumber,
    IconEllipsis,
    IconInfo,
    IconPlus,
    IconTrash,
    IconUsers,
    Loading,
    LoadingScreen,
    PageHeading,
    ToggleGroup,
    ToggleGroupItem,
    useDocumentTitle,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetPreferences } from "@/features/preference/preference_hooks";
import { CreateProjectPreferenceModal } from "@/features/preference/components/create_project_preference_modal";
import {
    PreferenceKind,
    ProjectPreference,
    RecipientPreference,
} from "@/features/preference/preference_type";
import { DeleteProjectPreferenceModal } from "../components/delete_project_preference_modal";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";
import { targetToString } from "@/lib/utils";

export function ProjectPreferenceList() {
    useDocumentTitle("Preferences  • Bodhveda");

    const id = useGetProjectIDFromParams();
    const [kind, setKind] = useState<PreferenceKind>("project");
    const isProject = kind === "project";

    const { data, isLoading, isFetching, isError } = useGetPreferences(
        id,
        kind
    );

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading preferences" />;
        }

        if (isLoading) {
            return <LoadingScreen />;
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
    }, [data, isError, isLoading, isProject]);

    return (
        <div>
            <PageHeading>
                <IconUsers size={18} />
                <h1>Preferences</h1>
                {isFetching && <Loading />}
            </PageHeading>

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

                {isProject && (
                    <CreateProjectPreferenceModal
                        renderTrigger={() => (
                            <Button>
                                <IconPlus size={16} />
                                Create Preference
                            </Button>
                        )}
                    />
                )}
            </div>

            {content}
        </div>
    );
}

function ActionCell({ preference }: { preference: ProjectPreference }) {
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
                <DeleteProjectPreferenceModal
                    open={deleteOpen}
                    setOpen={setDeleteOpen}
                    projectID={projectID}
                    preference={preference}
                />
            )}
        </>
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
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        id: "actions",
        cell: ({ row }) => <ActionCell preference={row.original} />,
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
        accessorKey: "created_at",
        header: () => <DataTableColumnHeader title="Created" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
    },
    {
        accessorKey: "updated_at",
        header: () => <DataTableColumnHeader title="Updated At" />,
        cell: ({ row }) =>
            formatDate(new Date(row.original.created_at), { time: true }),
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
