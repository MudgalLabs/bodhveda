import { createFileRoute } from "@tanstack/react-router";

import { NotificationList } from "@/features/notification/list/notifications_list";

export const Route = createFileRoute("/projects/$id/notifications")({
    component: NotificationList,
});
