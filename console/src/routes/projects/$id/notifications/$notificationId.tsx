import { createFileRoute } from "@tanstack/react-router";

import { DirectNotificationDetail } from "@/features/notification/detail/notification_detail";

// A DIRECT notification's detail page.
//
// ⚠️ Broadcasts get their OWN route (/broadcasts/$broadcastId) rather than
// sharing this one with a kind discriminator: notification.id and broadcast.id
// are independent SERIAL sequences whose ranges overlap, so a single
// /notifications/:id path cannot tell them apart. Both routes render the same
// detail component, so the two views never drift apart.
export const Route = createFileRoute(
    "/projects/$id/notifications/$notificationId"
)({
    component: RouteComponent,
});

function RouteComponent() {
    const { notificationId } = Route.useParams();

    return <DirectNotificationDetail notificationID={notificationId} />;
}
