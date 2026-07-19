import { createFileRoute, redirect } from "@tanstack/react-router";

// There's no longer a projects list page. Bare `/projects` just bounces to the
// landing resolver, which sends the user into a project (or the create-first
// screen).
export const Route = createFileRoute("/projects/")({
    beforeLoad: () => {
        throw redirect({ to: "/" });
    },
});
