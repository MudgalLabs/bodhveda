import { createFileRoute } from "@tanstack/react-router";

import { BroadcastNotificationDetail } from "@/features/notification/detail/notification_detail";

// A BROADCAST's detail page. Separate from /notifications/$notificationId
// because the two id spaces overlap — see the note there. Renders the same
// component.
export const Route = createFileRoute("/projects/$id/broadcasts/$broadcastId")({
    component: RouteComponent,
});

function RouteComponent() {
    const { broadcastId } = Route.useParams();

    return <BroadcastNotificationDetail broadcastID={broadcastId} />;
}
