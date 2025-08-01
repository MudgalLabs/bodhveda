import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";

import { useAuth } from "@/features/auth/auth_context";
import { Button } from "@/components/button";

export const Route = createFileRoute("/")({
    component: Index,
    beforeLoad: ({ context, location }) => {
        if (!context.auth.isAuthenticated) {
            throw redirect({
                to: "/login",
                search: {
                    redirect: location.href ? location.href : undefined,
                },
            });
        }
    },
});

function Index() {
    const { user, logout } = useAuth();

    const navigate = useNavigate();

    if (!user) {
        navigate({
            to: "/login",
        });
    }

    return (
        <div>
            <p>You are Authenticated! - {user?.email}</p>

            <Button onClick={() => logout()}>Log out</Button>
        </div>
    );
}
