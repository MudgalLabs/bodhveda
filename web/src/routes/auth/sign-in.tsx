import { createFileRoute, redirect } from "@tanstack/react-router";

import { ContinueWithGoogle } from "@/components/continue_with_google";

export const Route = createFileRoute("/auth/sign-in")({
    component: Login,
    beforeLoad: ({ context }) => {
        if (context.auth.isAuthenticated) {
            throw redirect({
                to: "/",
            });
        }
    },
});

function Login() {
    return (
        <div className="h-screen flex-center">
            <ContinueWithGoogle />
        </div>
    );
}
