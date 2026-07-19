import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
    Button,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
    DropdownMenuTrigger,
    IconCheck,
    IconChevronsUpDown,
    IconEdit,
    IconEllipsis,
    IconPlus,
    IconTrash,
} from "netra";

import { cn } from "@/lib/utils";
import { DEFAULT_RANGE_PRESET } from "@/features/dashboard/analytics_range";
import { useGetProjects } from "@/features/project/project_hooks";
import { ProjectListItem } from "@/features/project/project_types";
import { CreateProjectModal } from "@/features/project/components/create_project_modal";
import { EditProjectModal } from "@/features/project/components/edit_project_modal";
import { DeleteProjectModal } from "@/features/project/components/delete_project_modal";

interface ProjectSwitcherProps {
    // Internal id of the project currently being viewed.
    currentId: string;
    // Whether the sidebar is expanded. When collapsed we render an icon-only trigger.
    open: boolean;
}

export function ProjectSwitcher(props: ProjectSwitcherProps) {
    const { currentId, open } = props;

    const navigate = useNavigate();
    const { data: projects } = useGetProjects();

    // The project whose edit/delete modal is currently open (null = none). The
    // modals live outside the DropdownMenu so they survive it closing on select.
    const [editTarget, setEditTarget] = useState<ProjectListItem | null>(null);
    const [deleteTarget, setDeleteTarget] = useState<ProjectListItem | null>(
        null
    );

    const list = projects?.data ?? [];
    const current = list.find((p) => String(p.id) === currentId);

    const goToProject = (id: number) => {
        navigate({
            to: "/projects/$id/dashboard",
            params: { id: String(id) },
            search: { preset: DEFAULT_RANGE_PRESET },
        });
    };

    return (
        <>
            <div
                className={cn(
                    "flex gap-1",
                    open ? "items-center" : "flex-col"
                )}
            >
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        {open ? (
                            <Button
                                variant="secondary"
                                className="min-w-0 flex-1 justify-between gap-2"
                            >
                                <span className="truncate">
                                    {current?.name ?? "Select project"}
                                </span>
                                <IconChevronsUpDown
                                    size={16}
                                    className="text-text-muted shrink-0"
                                />
                            </Button>
                        ) : (
                            <Button
                                variant="secondary"
                                size="icon"
                                aria-label="Switch project"
                                className="h-9 w-9"
                            >
                                <IconChevronsUpDown size={16} />
                            </Button>
                        )}
                    </DropdownMenuTrigger>

                    <DropdownMenuContent className="w-[196px]">
                        {list.map((project) => {
                            const selected = String(project.id) === currentId;

                            return (
                                <div
                                    key={project.id}
                                    className="flex items-center gap-1"
                                >
                                    <DropdownMenuItem
                                        className="min-w-0 flex-1"
                                        onSelect={() => {
                                            if (!selected) {
                                                goToProject(project.id);
                                            }
                                        }}
                                    >
                                        <IconCheck
                                            size={16}
                                            className={cn(
                                                "shrink-0",
                                                !selected && "invisible"
                                            )}
                                        />
                                        <span className="truncate">
                                            {project.name}
                                        </span>
                                    </DropdownMenuItem>

                                    <DropdownMenuSub>
                                        <DropdownMenuSubTrigger className="px-2 [&>svg:last-child]:hidden">
                                            <IconEllipsis size={16} />
                                        </DropdownMenuSubTrigger>
                                        <DropdownMenuSubContent>
                                            <DropdownMenuItem
                                                onSelect={() =>
                                                    setEditTarget(project)
                                                }
                                            >
                                                <IconEdit size={16} />
                                                Rename
                                            </DropdownMenuItem>
                                            <DropdownMenuItem
                                                className="text-text-destructive"
                                                onSelect={() =>
                                                    setDeleteTarget(project)
                                                }
                                            >
                                                <IconTrash size={16} />
                                                Delete
                                            </DropdownMenuItem>
                                        </DropdownMenuSubContent>
                                    </DropdownMenuSub>
                                </div>
                            );
                        })}

                    </DropdownMenuContent>
                </DropdownMenu>

                <CreateProjectModal
                    onCreated={(project) => goToProject(project.id)}
                    renderTrigger={() => (
                        <Button
                            variant="secondary"
                            size="icon"
                            aria-label="Create project"
                            className="h-9 w-9 shrink-0"
                        >
                            <IconPlus size={16} />
                        </Button>
                    )}
                />
            </div>

            {editTarget && (
                <EditProjectModal
                    open={editTarget !== null}
                    setOpen={(o) => !o && setEditTarget(null)}
                    id={editTarget.id}
                    name={editTarget.name}
                />
            )}

            {deleteTarget && (
                <DeleteProjectModal
                    open={deleteTarget !== null}
                    setOpen={(o) => !o && setDeleteTarget(null)}
                    id={deleteTarget.id}
                    name={deleteTarget.name}
                    onDeleted={() => {
                        // If the user deleted the project they were viewing,
                        // send them to the resolver which picks another (or the
                        // create-your-first-project screen).
                        if (String(deleteTarget.id) === currentId) {
                            navigate({ to: "/" });
                        }
                    }}
                />
            )}
        </>
    );
}
