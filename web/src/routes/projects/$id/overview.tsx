import { createFileRoute } from "@tanstack/react-router";
import { PageHeading } from "netra";

export const Route = createFileRoute("/projects/$id/overview")({
    component: RouteComponent,
});

function RouteComponent() {
    return (
        <>
            <PageHeading heading="Overview" />
        </>
    );
}
