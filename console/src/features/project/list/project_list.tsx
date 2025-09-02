import { FormEvent, useMemo, useState } from "react";
import {
    Button,
    Card,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
    ErrorMessage,
    formatNumber,
    IconBell,
    IconEllipsis,
    IconPlus,
    IconTrash,
    IconUsers,
    Loading,
    LoadingScreen,
    PageHeading,
    Tooltip,
    useDocumentTitle,
} from "netra";
import { Link } from "@tanstack/react-router";

import { CreateProjectModal } from "@/features/project/list/create_project_modal";
import { useGetProjects } from "@/features/project/project_hooks";
import { DeleteProjectModal } from "@/features/project/components/delete_project_modal";

export function ProjectList() {
    useDocumentTitle("Projects  • Bodhveda");

    const { data, isLoading, isError } = useGetProjects();

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading projects" />;
        }

        if (isLoading) {
            return (
                <div className="h-screen w-screen">
                    <LoadingScreen />
                </div>
            );
        }

        if (!data) return null;

        return (
            <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
                {data?.data.map((project) => (
                    <div
                        key={project.id}
                        className="relative w-full h-full flex items-center justify-center"
                    >
                        {/* Dropdown menu is outside the Link and absolutely positioned */}
                        <div className="absolute top-2 right-2 z-10">
                            <ProjectOptionsDropdownMenu
                                id={project.id}
                                name={project.name}
                            />
                        </div>
                        <Link
                            to={`/projects/$id/home`}
                            params={{ id: String(project.id) }}
                            className="link-unstyled w-full h-full flex items-center justify-center"
                        >
                            <Card className="hover:border-border-hover w-full sm:w-72 h-36 flex-center smooth-colors relative">
                                <div className="flex flex-col items-center gap-y-4">
                                    <h2 className="text-lg font-semibold">
                                        {project.name}
                                    </h2>

                                    <div className="flex-x gap-x-4">
                                        <Tooltip
                                            content="Total Recipients"
                                            contentProps={{
                                                side: "left",
                                            }}
                                        >
                                            <div className="flex-x gap-x-1!">
                                                <IconUsers />
                                                {formatNumber(
                                                    project.total_recipients
                                                )}
                                            </div>
                                        </Tooltip>

                                        <Tooltip
                                            content="Total Notifications"
                                            contentProps={{
                                                side: "right",
                                            }}
                                        >
                                            <div className="flex-x gap-x-1!">
                                                <IconBell />
                                                {formatNumber(
                                                    project.total_notifications
                                                )}
                                            </div>
                                        </Tooltip>
                                    </div>
                                </div>
                            </Card>
                        </Link>
                    </div>
                ))}
            </div>
        );
    }, [data, isError, isLoading]);

    return (
        <div className="w-full max-w-[1200px] mx-auto mt-12 px-4">
            <PageHeading hideSidebarToggle>
                <div className="flex-x justify-between w-full">
                    <div>
                        <h1>Projects</h1>
                        {isLoading && <Loading />}
                    </div>

                    <div className="flex justify-end">
                        <CreateProjectModal
                            renderTrigger={() => (
                                <Button>
                                    <IconPlus size={16} />
                                    Create Project
                                </Button>
                            )}
                        />
                    </div>
                </div>
            </PageHeading>

            {content}
        </div>
    );
}

function ProjectOptionsDropdownMenu(props: { id: number; name: string }) {
    const { id, name } = props;

    const [dropdownOpen, setDropdownOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);

    const handleOpenDeleteConfirm = (e: FormEvent) => {
        e.stopPropagation();
        e.preventDefault();
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
                <DeleteProjectModal
                    open={deleteOpen}
                    setOpen={setDeleteOpen}
                    id={id}
                    name={name}
                />
            )}
        </>
    );
}
