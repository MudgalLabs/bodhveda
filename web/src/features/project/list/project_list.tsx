import {
    Button,
    Card,
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
    ErrorMessage,
    IconEllipsis,
    IconPlus,
    IconTrash,
    LoadingScreen,
    PageHeading,
} from "netra";
import { Link } from "@tanstack/react-router";

import { CreateProjectModal } from "@/features/project/list/create_project_modal";
import { useGetProjects } from "@/features/project/project_hooks";

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
                    <Card
                        key={project.id}
                        className="hover:border-border-hover w-full sm:w-72 h-36 flex-center smooth-colors cursor-pointer relative"
                    >
                        <ProjectOptionsDropdownMenu />

                        <Link
                            to={`/projects/$id/overview`}
                            params={{ id: String(project.id) }}
                            className="link-unstyled w-full h-full flex items-center justify-center"
                        >
                            <h2 className="text-lg font-semibold">
                                {project.name}
                            </h2>
                        </Link>
                    </Card>
                ))}
            </div>
        </div>
    );
}

function ProjectOptionsDropdownMenu() {
    return (
        <DropdownMenu>
            <DropdownMenuTrigger className="absolute top-2 right-2" asChild>
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
    );
}
