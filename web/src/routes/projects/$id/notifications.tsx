import { NotificationList } from "@/features/notification/list/notification_list";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/projects/$id/notifications")({
    component: NotificationList,
});
