import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/projects/$id/overview")({
    component: RouteComponent,
});

function RouteComponent() {
    return <div>Hello "/projects/$id/overview"!</div>;
}
