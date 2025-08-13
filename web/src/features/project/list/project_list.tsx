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
    LoadingScreen,
    PageHeading,
    toast,
    Tooltip,
} from "netra";
import { Link } from "@tanstack/react-router";

import { CreateProjectModal } from "@/features/project/list/create_project_modal";
import {
    useDeleteProject,
    useGetProjects,
} from "@/features/project/project_hooks";

export function ProjectList() {
    const { data, isLoading, isError } = useGetProjects();

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

    return (
        <div className="w-full max-w-[1200px] mx-auto mt-12 px-4">
            <PageHeading heading="Projects" />

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

            <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
                {data?.data.map((project) => (
                    <Link
                        to={`/projects/$id/home`}
                        params={{ id: String(project.id) }}
                        className="link-unstyled w-full h-full flex items-center justify-center"
                    >
                        <Card
                            key={project.id}
                            className="hover:border-border-hover w-full sm:w-72 h-36 flex-center smooth-colors relative"
                        >
                            <ProjectOptionsDropdownMenu
                                projectID={project.id}
                            />

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
                ))}
            </div>
        </div>
    );
}

function ProjectOptionsDropdownMenu(props: { projectID: number }) {
    const { projectID } = props;
    const { mutate, isPending } = useDeleteProject({
        onSuccess: () => {
            toast.success("Project deleted successfully");
        },
    });

    return (
        <DropdownMenu>
            <DropdownMenuTrigger className="absolute top-2 right-2" asChild>
                <Button variant="ghost" size="icon">
                    <IconEllipsis />
                </Button>
            </DropdownMenuTrigger>

            <DropdownMenuContent>
                <DropdownMenuItem
                    onClick={(e) => {
                        e.stopPropagation();
                        mutate(projectID);
                    }}
                    disabled={isPending}
                >
                    <IconTrash size={16} />
                    Delete
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}
