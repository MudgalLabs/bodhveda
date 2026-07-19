import { useEffect } from "react";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import {
    Button,
    ErrorMessage,
    IconPlus,
    LoadingScreen,
    useDocumentTitle,
} from "netra";

import { Branding } from "@/components/branding";
import { DEFAULT_RANGE_PRESET } from "@/features/dashboard/analytics_range";
import { CreateProjectModal } from "@/features/project/components/create_project_modal";
import { getLastProjectId } from "@/features/project/last_project";
import { useGetProjects } from "@/features/project/project_hooks";

export const Route = createFileRoute("/")({
    component: Index,
    beforeLoad: ({ context, location }) => {
        if (!context.auth.isAuthenticated) {
            throw redirect({
                to: "/auth/sign-in",
                search: {
                    redirect: location.href ? location.href : undefined,
                },
            });
        }
    },
});

// This is the app's landing route. There's no longer a projects list page, so
// it resolves the user straight into a project: it sends them to their first
// project's dashboard, or shows a "create your first project" screen when they
// have none.
function Index() {
    useDocumentTitle("Bodhveda");

    const navigate = useNavigate();
    const { data, isLoading, isError } = useGetProjects();

    const projects = data?.data ?? [];
    // Prefer the last project the user was in; fall back to their first project.
    const lastId = getLastProjectId();
    const targetProjectId =
        projects.find((p) => String(p.id) === lastId)?.id ?? projects[0]?.id;

    useEffect(() => {
        if (targetProjectId !== undefined) {
            navigate({
                to: "/projects/$id/dashboard",
                params: { id: String(targetProjectId) },
                search: { preset: DEFAULT_RANGE_PRESET },
                replace: true,
            });
        }
    }, [targetProjectId, navigate]);

    if (isError) {
        return (
            <div className="flex h-screen w-screen items-center justify-center">
                <ErrorMessage errorMsg="Error loading projects" />
            </div>
        );
    }

    // Still loading, or we have a project and are redirecting to it.
    if (isLoading || targetProjectId !== undefined) {
        return (
            <div className="h-screen w-screen">
                <LoadingScreen />
            </div>
        );
    }

    // No projects yet — prompt the user to create their first one.
    return (
        <div className="flex h-screen w-screen flex-col items-center justify-center gap-8 px-4">
            <Branding size="large" />

            <div className="flex flex-col items-center gap-2 text-center">
                <h1>Create your first project</h1>
                <p className="text-text-muted max-w-md">
                    Create a project for your app to start sending
                    notifications.
                </p>
            </div>

            <CreateProjectModal
                onCreated={(project) =>
                    navigate({
                        to: "/projects/$id/dashboard",
                        params: { id: String(project.id) },
                        search: { preset: DEFAULT_RANGE_PRESET },
                        replace: true,
                    })
                }
                renderTrigger={() => (
                    <Button>
                        <IconPlus size={16} />
                        Create Project
                    </Button>
                )}
            />
        </div>
    );
}
