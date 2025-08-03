import { Button, Card, IconPlus, LoadingScreen } from "netra";
import {
    useCreateProject,
    useGetProjects,
} from "@/features/project/project_hooks";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

export function ProjectList() {
    const [showCreate, setShowCreate] = useState(false);
    const [name, setName] = useState("");

    const { data, isLoading, isError } = useGetProjects();
    const { mutate: createProject } = useCreateProject();

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (name.trim() === "") return;

        createProject(
            { name },
            {
                onSuccess: () => {
                    setName("");
                    setShowCreate(false);
                },
            }
        );
    };

    if (isError) {
        return (
            <div className="text-text-destructive">Error loading projects</div>
        );
    }

    if (isLoading) {
        return <LoadingScreen />;
    }

    return (
        <div className="w-full max-w-[1200px] mx-auto mt-12 px-4">
            <div className="flex-x justify-between">
                <h1 className="big-heading">Projects</h1>
                <Button onClick={() => setShowCreate((prev) => !prev)}>
                    <IconPlus size={16} />
                    Create Project
                </Button>
            </div>

            {showCreate && (
                <div className="mt-4">
                    <form onSubmit={handleSubmit}>
                        <input
                            type="text"
                            placeholder="Project Name"
                            className="input"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                        />
                        <Button type="submit" className="ml-2">
                            Create
                        </Button>
                    </form>
                </div>
            )}

            <div className="mt-8">
                {data?.data.map((project) => (
                    <Link
                        key={project.id}
                        to={`/projects/$id/overview`}
                        params={{ id: String(project.id) }}
                        className="link-unstyled"
                    >
                        <Card className="hover:border-border-hover w-72 h-36 flex-center smooth-colors">
                            <h2 className="text-lg font-semibold">
                                {project.name}
                            </h2>
                        </Card>
                    </Link>
                ))}
            </div>
        </div>
    );
}
