import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";

import { useAuth } from "@/features/auth/auth_context";

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

function Index() {
    const { isAuthenticated } = useAuth();

    const navigate = useNavigate();

    if (!isAuthenticated) {
        navigate({
            to: "/auth/sign-in",
        });
    } else {
        navigate({
            to: "/projects",
        });
    }

    return null;
}
