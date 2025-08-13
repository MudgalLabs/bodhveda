import { createFileRoute } from "@tanstack/react-router";
import { RecipientList } from "@/features/recipient/list/recipient_list";

export const Route = createFileRoute("/projects/$id/recipients")({
    component: RecipientList,
});
