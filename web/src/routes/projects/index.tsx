import { createFileRoute, redirect } from "@tanstack/react-router";

import { ProjectList } from "@/features/project/list/project_list";

export const Route = createFileRoute("/projects/")({
    component: ProjectList,
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
