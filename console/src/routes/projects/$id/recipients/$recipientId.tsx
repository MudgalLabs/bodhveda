import { createFileRoute } from "@tanstack/react-router";

import { RecipientDetail } from "@/features/recipient/detail/recipient_detail";

export const Route = createFileRoute("/projects/$id/recipients/$recipientId")({
    component: RouteComponent,
});

function RouteComponent() {
    // `recipientId` is the customer-chosen external_id, which may contain
    // URL-hostile characters. The router decodes the param for us — what comes
    // out here is the raw id, and API_ROUTES re-encodes it per request.
    const { recipientId } = Route.useParams();

    return <RecipientDetail recipientID={recipientId} />;
}
