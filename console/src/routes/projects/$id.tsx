import { Outlet, createFileRoute } from "@tanstack/react-router";
import { AppLayout } from "@/components/app_layout";

export const Route = createFileRoute("/projects/$id")({
    component: () => (
        <AppLayout>
            <Outlet />
        </AppLayout>
    ),
});
