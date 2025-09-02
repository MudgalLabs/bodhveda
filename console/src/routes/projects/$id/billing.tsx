import { createFileRoute } from "@tanstack/react-router";

import { Billing } from "@/features/billing/billing";

export const Route = createFileRoute("/projects/$id/billing")({
    component: Billing,
});
