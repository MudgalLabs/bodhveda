import { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import {
    Button,
    DataTable,
    DataTableColumnHeader,
    DataTablePagination,
    DataTableSmart,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
    ErrorMessage,
    formatDate,
    formatNumber,
    IconEdit,
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
    mediumLabel,
    PreferenceKind,
    ProjectPreference,
    RecipientPreference,
} from "@/features/preference/preference_type";
import { DeleteProjectPreferenceModal } from "../components/delete_project_preference_modal";
import { EditProjectPreferenceModal } from "../components/edit_project_preference_modal";
import { TargetInfoTooltip } from "@/components/target_info_tooltip";
import { RecipientLink } from "@/features/recipient/recipient_link";
import { targetToString } from "@/lib/utils";

interface ProjectPreferenceListProps {
    /** The kind being viewed. Owned by the route, which reads it from the URL. */
    kind: PreferenceKind;
    onKindChange: (kind: PreferenceKind) => void;
}

export function ProjectPreferenceList({
    kind,
    onKindChange,
}: ProjectPreferenceListProps) {
    useDocumentTitle("Preferences  • Bodhveda");

    const id = useGetProjectIDFromParams();
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
                        value && onKindChange(value as PreferenceKind)
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
    const [editOpen, setEditOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);

    const handleOpenEdit = () => {
        setDropdownOpen(false);
        setEditOpen(true);
    };

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
                            variant="ghost"
                            className="w-full! justify-start!"
                            onClick={handleOpenEdit}
                        >
                            <IconEdit size={16} />
                            Edit
                        </Button>
                    </DropdownMenuItem>

                    <DropdownMenuItem asChild>
                        <Button
                            variant="destructive"
                            className="w-full! justify-start!"
                            onClick={handleOpenDeleteConfirm}
                        >
                            <IconTrash size={16} />
                            Delete
                        </Button>
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>

            {editOpen && (
                <EditProjectPreferenceModal
                    open={editOpen}
                    setOpen={setEditOpen}
                    projectID={projectID}
                    preference={preference}
                />
            )}

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
        accessorKey: "name",
        header: () => <DataTableColumnHeader title="Name" />,
    },
    {
        accessorKey: "description",
        header: () => <DataTableColumnHeader title="Description" />,
        cell: ({ row }) => row.original.description || "—",
    },
    {
        accessorKey: "medium",
        header: () => <DataTableColumnHeader title="Medium" />,
        cell: ({ row }) => mediumLabel(row.original.medium),
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
        cell: ({ row }) => (
            <RecipientLink recipientID={row.original.recipient_id} />
        ),
    },
    {
        accessorKey: "medium",
        header: () => <DataTableColumnHeader title="Medium" />,
        cell: ({ row }) => mediumLabel(row.original.medium),
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
            formatDate(new Date(row.original.updated_at), { time: true }),
    },
];

interface RecipientPreferenceListTableProps {
    data: RecipientPreference[];
}

function RecipientPreferenceListTable({
    data,
}: RecipientPreferenceListTableProps) {
    // Default sort: most-recently-updated first, so a just-changed preference
    // (e.g. a fresh opt-out) surfaces at the top instead of being buried on a
    // later page. netra's table flips to manual/server sorting the moment you
    // pass `state.sorting`, so we pre-sort here and let it paginate
    // client-side; column-header clicks still re-sort client-side.
    const sorted = useMemo(
        () =>
            [...data].sort(
                (a, b) =>
                    new Date(b.updated_at).getTime() -
                    new Date(a.updated_at).getTime()
            ),
        [data]
    );

    return (
        <DataTableSmart data={sorted} columns={recipientPreferenceColumns}>
            {(table) => (
                <div className="space-y-4">
                    <DataTable table={table} />
                    {sorted.length > 10 && (
                        <DataTablePagination
                            table={table}
                            total={sorted.length}
                        />
                    )}
                </div>
            )}
        </DataTableSmart>
    );
}
